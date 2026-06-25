package ecs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/devsy-org/devsy-provider-ecs/pkg/options"
	"github.com/devsy-org/devsy/pkg/devcontainer/config"
	"github.com/devsy-org/devsy/pkg/driver"
	"github.com/devsy-org/log"
)

const statusRunning = "running"

type EcsProvider struct {
	Config    *options.Options
	AwsConfig aws.Config
	Log       log.Logger

	client *ecs.Client
}

func NewProvider(
	ctx context.Context,
	options *options.Options,
	logs log.Logger,
) (*EcsProvider, error) {
	cfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	// create provider
	provider := &EcsProvider{
		Config:    options,
		AwsConfig: cfg,
		Log:       logs,

		client: ecs.NewFromConfig(cfg),
	}

	return provider, nil
}

func (p *EcsProvider) TargetArchitecture(ctx context.Context, workspaceId string) (string, error) {
	return p.Config.ClusterArchitecture, nil
}

func (p *EcsProvider) StartTask(ctx context.Context, workspaceId string) error {
	// noop operation if running on fargate
	if p.Config.LaunchType == string(types.LaunchTypeFargate) {
		return nil
	}

	return p.startTask(ctx, workspaceId)
}

func (p *EcsProvider) StopTask(ctx context.Context, workspaceId string) error {
	// noop operation if running on fargate
	if p.Config.LaunchType == string(types.LaunchTypeFargate) {
		return nil
	}

	// stop the task
	return p.stopTask(ctx, workspaceId)
}

func (p *EcsProvider) RunTask(
	ctx context.Context,
	workspaceId string,
	runOptions *driver.RunOptions,
) error {
	err := p.registerTaskDefinition(ctx, workspaceId, runOptions)
	if err != nil {
		return err
	}

	err = p.startTask(ctx, workspaceId)
	if err != nil {
		_ = p.stopTask(ctx, workspaceId)
		_ = p.deleteTaskDefinition(ctx, workspaceId)
		return err
	}

	return nil
}

func (p *EcsProvider) FindTask(
	ctx context.Context,
	workspaceId string,
) (*config.ContainerDetails, error) {
	task, err := p.getTaskID(ctx, workspaceId)
	if err != nil {
		return nil, err
	} else if task == nil {
		return nil, nil
	}

	// get labels
	taskDefinition, err := p.client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: task.TaskDefinitionArn,
	})
	if err != nil {
		return nil, fmt.Errorf("describe task definition: %w", err)
	}
	labels := taskDefinition.TaskDefinition.ContainerDefinitions[0].DockerLabels

	return &config.ContainerDetails{
		ID:      *task.TaskArn,
		Created: task.CreatedAt.String(),
		State: config.ContainerDetailsState{
			Status:    taskStatus(task),
			StartedAt: taskStartedAt(task),
		},
		Config: config.ContainerDetailsConfig{
			Labels: labels,
		},
	}, nil
}

func (p *EcsProvider) DeleteTask(ctx context.Context, workspaceId string) error {
	// stop the task
	err := p.stopTask(ctx, workspaceId)
	if err != nil {
		return err
	}

	// TODO: delete ecs volume?

	// delete task definition
	err = p.deleteTaskDefinition(ctx, workspaceId)
	if err != nil {
		return err
	}

	return nil
}

func (p *EcsProvider) stopTask(ctx context.Context, workspaceId string) error {
	// stop the task
	task, err := p.getTaskID(ctx, workspaceId)
	if err != nil {
		return err
	} else if task != nil {
		// delete the task
		p.Log.Infof("Stopping task...")
		_, err = p.client.StopTask(ctx, &ecs.StopTaskInput{
			Task:    task.TaskArn,
			Cluster: options.Ptr(p.Config.ClusterID),
		})
		if err != nil {
			return fmt.Errorf("stop task: %w", err)
		}
	}

	return nil
}

func (p *EcsProvider) getTaskID(ctx context.Context, workspaceId string) (*types.Task, error) {
	runningTaskArns, err := p.client.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       options.Ptr(p.Config.ClusterID),
		Family:        options.Ptr("devsy-" + workspaceId),
		DesiredStatus: types.DesiredStatusRunning,
		MaxResults:    options.Ptr(int32(10)),
	})
	if err != nil {
		return nil, fmt.Errorf("list running tasks: %w", err)
	}

	// search stopped if there is no desired running
	taskArns := runningTaskArns.TaskArns
	if len(taskArns) == 0 {
		stoppedTaskArns, err := p.client.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       options.Ptr(p.Config.ClusterID),
			Family:        options.Ptr("devsy-" + workspaceId),
			DesiredStatus: types.DesiredStatusStopped,
			MaxResults:    options.Ptr(int32(10)),
		})
		if err != nil {
			return nil, fmt.Errorf("list stopped tasks: %w", err)
		}

		taskArns = stoppedTaskArns.TaskArns
	}
	if len(taskArns) == 0 {
		return nil, nil
	}

	// get tasks
	tasks, err := p.client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Tasks:   taskArns,
		Cluster: options.Ptr(p.Config.ClusterID),
	})
	switch {
	case err != nil:
		return nil, fmt.Errorf("describe tasks: %w", err)
	case len(tasks.Failures) > 0:
		return nil, fmt.Errorf("describe tasks failures: %s", *tasks.Failures[0].Reason)
	case len(tasks.Tasks) == 0:
		return nil, nil
	}

	// sort tasks by revision
	sort.SliceStable(tasks.Tasks, func(i, j int) bool {
		return tasks.Tasks[i].CreatedAt.Unix() > tasks.Tasks[j].CreatedAt.Unix()
	})

	return &tasks.Tasks[0], nil
}

func (p *EcsProvider) startTask(ctx context.Context, workspaceId string) error {
	taskDefinitionID, err := p.getTaskDefinitionArn(ctx, workspaceId)
	if err != nil {
		return err
	}

	securityGroups := []string{}
	if p.Config.SecurityGroupID != "" {
		securityGroups = append(securityGroups, p.Config.SecurityGroupID)
	}

	p.Log.Infof("Running Task...")
	taskOutput, err := p.client.RunTask(ctx, &ecs.RunTaskInput{
		TaskDefinition:       options.Ptr(taskDefinitionID),
		Cluster:              options.Ptr(p.Config.ClusterID),
		Count:                options.Ptr(int32(1)),
		EnableExecuteCommand: true,
		LaunchType:           types.LaunchType(p.Config.LaunchType),
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        []string{p.Config.SubnetID},
				SecurityGroups: securityGroups,
				AssignPublicIp: types.AssignPublicIp(p.Config.AssignPublicIp),
			},
		},
		Tags: getTags(workspaceId),
	})
	if err != nil {
		return fmt.Errorf("run task: %w", err)
	} else if len(taskOutput.Failures) > 0 {
		return fmt.Errorf("run task failure: %w", errors.New(*taskOutput.Failures[0].Reason))
	}

	return p.waitForTaskRunning(ctx, workspaceId)
}

func (p *EcsProvider) waitForTaskRunning(ctx context.Context, workspaceId string) error {
	timeout := time.Minute * 5
	now := time.Now()
	for time.Since(now) < timeout {
		p.Log.Infof("Waiting for Task to become running...")
		task, err := p.getTaskID(ctx, workspaceId)
		if err != nil {
			return fmt.Errorf("error retrieving task: %w", err)
		}

		running, err := taskRunning(task)
		if err != nil {
			return err
		} else if running {
			p.Log.Info("Task successfully started")
			return nil
		}

		time.Sleep(time.Second * 5)
	}

	return errors.New("run task failed, timed out waiting for task to be running")
}

// taskRunning reports whether the task has reached the running state. It returns
// an error if the task was stopped, and (false, nil) while still pending.
func taskRunning(task *types.Task) (bool, error) {
	if task == nil {
		return false, nil
	}

	if task.DesiredStatus != nil && strings.ToLower(*task.DesiredStatus) != statusRunning {
		if task.StoppedReason != nil {
			return false, fmt.Errorf("run task failed, task was stopped: %s", *task.StoppedReason)
		}

		return false, errors.New("run task failed, task was stopped without a reason")
	}

	if task.LastStatus != nil && strings.ToLower(*task.LastStatus) == statusRunning {
		return true, nil
	}

	return false, nil
}

func taskStatus(task *types.Task) string {
	if task.LastStatus == nil {
		return "created"
	}

	switch strings.ToUpper(*task.LastStatus) {
	case string(types.DesiredStatusRunning):
		return statusRunning
	case string(types.DesiredStatusStopped):
		return "exited"
	default:
		return "created"
	}
}

func taskStartedAt(task *types.Task) string {
	if task.StartedAt != nil {
		return task.StartedAt.String()
	}

	return ""
}

package options

import (
	"fmt"
	"os"
)

var DefaultSSHPort int = 19583

type Options struct {
	DevContainerID string

	ClusterID           string
	ClusterArchitecture string

	SubnetID        string
	SecurityGroupID string

	TaskRoleARN      string
	ExecutionRoleARN string

	TaskCpu    string
	TaskMemory string

	LaunchType     string
	AssignPublicIp string
}

func FromEnv() (*Options, error) {
	retOptions := &Options{}

	// required
	required := []struct {
		name string
		dest *string
	}{
		{"DEVCONTAINER_ID", &retOptions.DevContainerID},
		{"CLUSTER_ID", &retOptions.ClusterID},
		{"SUBNET_ID", &retOptions.SubnetID},
		{"CLUSTER_ARCHITECTURE", &retOptions.ClusterArchitecture},
		{"TASK_CPU", &retOptions.TaskCpu},
		{"TASK_MEMORY", &retOptions.TaskMemory},
		{"LAUNCH_TYPE", &retOptions.LaunchType},
		{"ASSIGN_PUBLIC_IP", &retOptions.AssignPublicIp},
	}
	for _, opt := range required {
		val, err := fromEnvOrError(opt.name)
		if err != nil {
			return nil, err
		}

		*opt.dest = val
	}

	// optional
	retOptions.SecurityGroupID = os.Getenv("SECURITY_GROUP_ID")
	retOptions.TaskRoleARN = os.Getenv("TASK_ROLE_ARN")
	retOptions.ExecutionRoleARN = os.Getenv("EXECUTION_ROLE_ARN")

	return retOptions, nil
}

func fromEnvOrError(name string) (string, error) {
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf(
			"couldn't find option %s in environment, please make sure %s is defined",
			name,
			name,
		)
	}

	return val, nil
}

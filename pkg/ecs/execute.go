package ecs

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/session-manager-plugin/src/datachannel"
	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	_ "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session/portsession"
	_ "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session/shellsession"
	"github.com/devsy-org/devsy-provider-ecs/pkg/options"
	devsylog "github.com/devsy-org/devsy/pkg/log"
	"github.com/devsy-org/devsy/pkg/ssh"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Stdio groups the three standard streams to keep argument lists small.
type Stdio struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// ExecRequest describes a command to execute inside a workspace container.
type ExecRequest struct {
	WorkspaceID string
	User        string
	Command     string
	Streams     Stdio
}

// execCommand bundles everything needed to run a command inside a container.
type execCommand struct {
	target  string
	user    string
	command string
	streams Stdio
}

func (p *EcsProvider) ExecuteCommand(ctx context.Context, req ExecRequest) error {
	task, err := p.getTaskID(ctx, req.WorkspaceID)
	if err != nil {
		return err
	} else if task == nil {
		return fmt.Errorf("no task for workspace %s found", req.WorkspaceID)
	}

	target := "ecs:" + getIDFromArn(p.Config.ClusterID) +
		"_" + getIDFromArn(*task.TaskArn) +
		"_" + *task.Containers[0].RuntimeId

	return executeCommand(ctx, execCommand{
		target:  target,
		user:    req.User,
		command: req.Command,
		streams: req.Streams,
	})
}

func executeCommand(ctx context.Context, ec execCommand) error {
	// create context
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = stdinWriter.Close() }()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = stdoutWriter.Close() }()

	tunnelChan := make(chan error, 1)
	go func() {
		writer := devsylog.Writer(devsylog.LevelInfo)
		defer func() { _ = writer.Close() }()

		tunnelChan <- startProxyCommand(cancelCtx, ec.target, Stdio{
			Stdin:  stdinReader,
			Stdout: stdoutWriter,
			Stderr: writer,
		})
	}()

	// connect to container
	containerChan := make(chan error, 1)
	go func() {
		// start ssh client as root / default user
		sshClient, err := ssh.StdioClientWithUser(stdoutReader, stdinWriter, ec.user, false)
		if err != nil {
			containerChan <- errors.Wrap(err, "create ssh client")
			return
		}

		defer func() { _ = sshClient.Close() }()
		defer cancel()

		containerChan <- ssh.Run(cancelCtx, ssh.RunOptions{
			Client:  sshClient,
			Command: ec.command,
			Stdin:   ec.streams.Stdin,
			Stdout:  ec.streams.Stdout,
			Stderr:  ec.streams.Stderr,
		})
	}()

	// wait for result
	select {
	case err := <-containerChan:
		return errors.Wrap(err, "ssh into container")
	case err := <-tunnelChan:
		return errors.Wrap(err, "connect to ssm")
	}
}

func startProxyCommand(ctx context.Context, target string, streams Stdio) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	// Re-invokes this provider's own binary (os.Executable) to open the tunnel,
	// so the command path is trusted rather than user-supplied.
	cmd := exec.CommandContext( // #nosec G204
		ctx,
		executable,
		"tunnel",
		"--target", target,
	)
	cmd.Stdin = streams.Stdin
	cmd.Stdout = streams.Stdout
	cmd.Stderr = streams.Stderr
	return cmd.Run()
}

func getIDFromArn(arn string) string {
	if !strings.HasPrefix(arn, "arn:") {
		return arn
	}

	taskArnSplitted := strings.Split(arn, "/")
	return taskArnSplitted[len(taskArnSplitted)-1]
}

func (p *EcsProvider) StartSession(target string, port int) error {
	out, err := ssm.NewFromConfig(p.AwsConfig).
		StartSession(context.Background(), &ssm.StartSessionInput{
			Target:       options.Ptr(target),
			DocumentName: options.Ptr("AWS-StartSSHSession"),
			Parameters: map[string][]string{
				"portNumber": {strconv.Itoa(port)},
			},
		})
	if err != nil {
		return err
	}

	ssmSession := new(session.Session)
	ssmSession.SessionId = *out.SessionId
	ssmSession.StreamUrl = *out.StreamUrl
	ssmSession.TokenValue = *out.TokenValue
	ssmSession.ClientId = uuid.NewString()
	ssmSession.TargetId = target
	ssmSession.DataChannel = &datachannel.DataChannel{}
	return ssmSession.Execute(log.Logger(false, ssmSession.ClientId))
}

package cmd

import (
	"context"
	"os"

	"github.com/devsy-org/devsy-provider-ecs/pkg/ecs"
	"github.com/devsy-org/devsy-provider-ecs/pkg/options"
	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
)

// CommandCmd holds the cmd flags.
type CommandCmd struct{}

// NewCommandCmd defines a command.
func NewCommandCmd() *cobra.Command {
	cmd := &CommandCmd{}
	commandCmd := &cobra.Command{
		Use:   "command",
		Short: "Command a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default.ErrorStreamOnly())
		},
	}

	return commandCmd
}

// Run runs the command logic.
func (cmd *CommandCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	ecsProvider, err := ecs.NewProvider(ctx, options, log)
	if err != nil {
		return err
	}

	return ecsProvider.ExecuteCommand(ctx, ecs.ExecRequest{
		WorkspaceID: options.DevContainerID,
		User:        os.Getenv("DEVCONTAINER_USER"),
		Command:     os.Getenv("DEVCONTAINER_COMMAND"),
		Streams: ecs.Stdio{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		},
	})
}

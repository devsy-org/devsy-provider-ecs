package cmd

import (
	"context"

	"github.com/devsy-org/devsy-provider-ecs/pkg/ecs"
	"github.com/devsy-org/devsy-provider-ecs/pkg/options"
	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
)

// StartCmd holds the cmd flags.
type StartCmd struct{}

// NewStartCmd defines a command.
func NewStartCmd() *cobra.Command {
	cmd := &StartCmd{}
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default)
		},
	}

	return startCmd
}

// Run runs the command logic.
func (cmd *StartCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	ecsProvider, err := ecs.NewProvider(ctx, options, log)
	if err != nil {
		return err
	}

	return ecsProvider.StartTask(ctx, options.DevContainerID)
}

package cmd

import (
	"context"

	"github.com/devsy-org/devsy-provider-ecs/pkg/ecs"
	"github.com/devsy-org/devsy-provider-ecs/pkg/options"
	"github.com/devsy-org/log"
	"github.com/spf13/cobra"
)

// DeleteCmd holds the cmd flags.
type DeleteCmd struct{}

// NewDeleteCmd defines a command.
func NewDeleteCmd() *cobra.Command {
	cmd := &DeleteCmd{}
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default)
		},
	}

	return deleteCmd
}

// Run runs the command logic.
func (cmd *DeleteCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	ecsProvider, err := ecs.NewProvider(ctx, options, log)
	if err != nil {
		return err
	}

	return ecsProvider.DeleteTask(ctx, options.DevContainerID)
}

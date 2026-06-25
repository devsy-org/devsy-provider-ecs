package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/devsy-org/devsy-provider-ecs/pkg/version"
	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

// BuildInfo carries version metadata injected at build time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// NewRootCmd returns a new root command.
func NewRootCmd() *cobra.Command {
	ecsCmd := &cobra.Command{
		Use:           "devsy-provider-ecs",
		Short:         "ECS Provider commands",
		SilenceErrors: true,
		SilenceUsage:  true,

		PersistentPreRunE: func(cobraCmd *cobra.Command, args []string) error {
			cfg := log.Config{Verbosity: 1}
			if os.Getenv("DEVSY_DEBUG") == "true" {
				cfg.Debug = true
			}
			log.Init(cfg)

			return nil
		},
	}

	return ecsCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(info BuildInfo) {
	if info.Version != "" {
		version.Version = info.Version
	}

	// build the root command
	rootCmd := BuildRoot(info)

	// execute command
	err := rootCmd.Execute()
	if err != nil {
		var sshExitErr *ssh.ExitError
		if errors.As(err, &sshExitErr) {
			os.Exit(sshExitErr.ExitStatus())
		}

		var execExitErr *exec.ExitError
		if errors.As(err, &execExitErr) {
			if len(execExitErr.Stderr) > 0 {
				log.Error(string(execExitErr.Stderr))
			}

			os.Exit(execExitErr.ExitCode())
		}
		log.Fatal(err)
	}
}

// BuildRoot creates a new root command with all subcommands attached.
func BuildRoot(info BuildInfo) *cobra.Command {
	rootCmd := NewRootCmd()
	rootCmd.Version = formatVersion(info)

	rootCmd.AddCommand(NewEntrypointCmd())
	rootCmd.AddCommand(NewTunnelCmd())
	rootCmd.AddCommand(NewFindCmd())
	rootCmd.AddCommand(NewDeleteCmd())
	rootCmd.AddCommand(NewStartCmd())
	rootCmd.AddCommand(NewRunCmd())
	rootCmd.AddCommand(NewCommandCmd())
	rootCmd.AddCommand(NewStopCmd())
	rootCmd.AddCommand(NewTargetArchitectureCmd())
	return rootCmd
}

func formatVersion(info BuildInfo) string {
	v := info.Version
	if v == "" {
		v = version.Version
	}
	if info.Commit != "" && info.Date != "" {
		return fmt.Sprintf("%s (commit %s, built %s)", v, info.Commit, info.Date)
	}
	return v
}

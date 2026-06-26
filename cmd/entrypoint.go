package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/devsy-org/devsy-provider-ecs/pkg/options"
	"github.com/devsy-org/devsy/pkg/log"
	sshserver "github.com/devsy-org/devsy/pkg/ssh/server"
	"github.com/spf13/cobra"
)

type EntrypointCmd struct {
	Entrypoint string
	Cmd        string

	Port int
}

// NewEntrypointCmd returns a new command.
func NewEntrypointCmd() *cobra.Command {
	cmd := &EntrypointCmd{}
	cobraCmd := &cobra.Command{
		Use:           "entrypoint",
		Short:         "Starts the container with an ssh server in the background",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return cmd.Run()
		},
	}

	cobraCmd.Flags().
		StringVar(&cmd.Entrypoint, "entrypoint", "", "Base64 encoded json string with an entrypoint to execute")
	cobraCmd.Flags().
		StringVar(&cmd.Cmd, "cmd", "", "Base64 encoded json string with cmd to execute")
	cobraCmd.Flags().
		IntVar(&cmd.Port, "port", options.DefaultSSHPort, "The default port to use for the ssh server")
	return cobraCmd
}

func (cmd *EntrypointCmd) Run() error {
	address := fmt.Sprintf("127.0.0.1:%d", cmd.Port)
	server, err := sshserver.NewServer(address, nil, nil, "", "")
	if err != nil {
		return err
	}

	log.Infof("Listen and serve on: %s", address)
	go func() {
		err = server.ListenAndServe()
		if err != nil {
			log.Fatalf("SSH server failed: %v", err)
		} else {
			log.Fatal("SSH server ended unexpectedly")
		}
	}()

	args, err := cmd.buildArgs()
	if err != nil {
		return err
	}

	// wait indefinitely when there is nothing to run
	if len(args) == 0 {
		select {}
	}

	// run entrypoint. The args are the workspace's own entrypoint/cmd, which this
	// command exists to execute, so launching them as a subprocess is intentional.
	entrypointCmd := exec.Command(args[0], args[1:]...) // #nosec G204
	entrypointCmd.Stdout = os.Stdout
	entrypointCmd.Stdin = os.Stdin
	entrypointCmd.Stderr = os.Stderr
	return entrypointCmd.Run()
}

func (cmd *EntrypointCmd) buildArgs() ([]string, error) {
	args := []string{}
	if cmd.Entrypoint != "" {
		entrypoint, err := decodeStrArray(cmd.Entrypoint)
		if err != nil {
			return nil, fmt.Errorf("decode entrypoint: %w", err)
		}

		args = append(args, entrypoint...)
	}
	if cmd.Cmd != "" {
		c, err := decodeStrArray(cmd.Cmd)
		if err != nil {
			return nil, fmt.Errorf("decode cmd: %w", err)
		}

		args = append(args, c...)
	}

	return args, nil
}

func decodeStrArray(payload string) ([]string, error) {
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}

	strArr := []string{}
	err = json.Unmarshal(decoded, &strArr)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", string(decoded), err)
	}

	return strArr, nil
}

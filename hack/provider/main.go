package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	providerName = "ecs"
	githubOwner  = "devsy-org"
	githubRepo   = "devsy-provider-ecs"
)

type Provider struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Icon        string            `yaml:"icon"`
	IconDark    string            `yaml:"iconDark"`
	Home        string            `yaml:"home"`
	Options     Options           `yaml:"options"`
	Agent       Agent             `yaml:"agent"`
	Exec        map[string]string `yaml:"exec"`
}

type Options map[string]Option

type Option struct {
	Description string   `yaml:"description,omitempty"`
	Required    bool     `yaml:"required,omitempty"`
	Default     string   `yaml:"default,omitempty"`
	Command     string   `yaml:"command,omitempty"`
	Enum        []string `yaml:"enum,omitempty"`
}

type Agent struct {
	ContainerInactivityTimeout string         `yaml:"containerInactivityTimeout"`
	Local                      bool           `yaml:"local"`
	Binaries                   map[string]any `yaml:"binaries"`
	// DockerlessIgnorePaths is kept for backwards compatibility with older agents.
	DockerlessIgnorePaths string       `yaml:"dockerlessIgnorePaths"`
	Dockerless            Dockerless   `yaml:"dockerless"`
	Driver                string       `yaml:"driver"`
	Custom                CustomDriver `yaml:"custom"`
}

type Dockerless struct {
	IgnorePaths string `yaml:"ignorePaths"`
}

type CustomDriver struct {
	FindDevContainer    string `yaml:"findDevContainer"`
	CommandDevContainer string `yaml:"commandDevContainer"`
	StartDevContainer   string `yaml:"startDevContainer"`
	StopDevContainer    string `yaml:"stopDevContainer"`
	RunDevContainer     string `yaml:"runDevContainer"`
	DeleteDevContainer  string `yaml:"deleteDevContainer"`
	TargetArchitecture  string `yaml:"targetArchitecture"`
}

type Binary struct {
	OS       string `yaml:"os"`
	Arch     string `yaml:"arch"`
	Path     string `yaml:"path"`
	Checksum string `yaml:"checksum"`
}

type buildConfig struct {
	version     string
	projectRoot string
	isRelease   bool
	checksums   map[string]string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("expected version as argument")
	}

	cfg, err := newBuildConfig(os.Args[1])
	if err != nil {
		return err
	}

	provider := buildProvider(cfg)

	output, err := yaml.Marshal(provider)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	_, err = os.Stdout.Write(output)
	return err
}

func newBuildConfig(version string) (*buildConfig, error) {
	checksums, err := parseChecksums("./dist/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("parse checksums: %w", err)
	}

	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		owner := getEnvOrDefault("GITHUB_OWNER", githubOwner)
		projectRoot = fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/%s",
			owner,
			githubRepo,
			version,
		)
	}

	isRelease := strings.Contains(projectRoot, "github.com") &&
		strings.Contains(projectRoot, "/releases/")

	return &buildConfig{
		version:     version,
		projectRoot: projectRoot,
		isRelease:   isRelease,
		checksums:   checksums,
	}, nil
}

func buildProvider(cfg *buildConfig) Provider {
	return Provider{
		Name:        providerName,
		Version:     cfg.version,
		Description: "Devsy on ECS",
		Icon:        "https://raw.githubusercontent.com/devsy-org/devsy/main/desktop/src/renderer/public/icons/providers/aws.svg",
		IconDark:    "https://raw.githubusercontent.com/devsy-org/devsy/main/desktop/src/renderer/public/icons/providers/aws.svg",
		Home:        "https://github.com/devsy-org/devsy",
		Options:     buildOptions(),
		Agent:       buildAgent(cfg),
		Exec: map[string]string{
			"command": "\"${DEVSY}\" helper sh -c \"${COMMAND}\"",
		},
	}
}

func buildOptions() Options {
	return Options{
		"CLUSTER_ID": {
			Description: "ECS Cluster ID either as ARN or ID",
			Required:    true,
		},
		"SUBNET_ID": {
			Description: "ECS Subnet ID either as ARN or ID to run the tasks in. " +
				"This can either be a private subnet with a NAT Gateway or a Public Subnet. " +
				"Depending on the type of the subnet you will need to set ASSIGN_PUBLIC_IP accordingly",
			Required: true,
		},
		"AWS_PROFILE": {
			Description: "The aws profile name to use",
			Command:     `printf "%s" "${AWS_PROFILE:-default}"`,
		},
		"TASK_ROLE_ARN": {
			Description: "ECS Task Role ARN to use for the task definition with IAM permissions " +
				"required for ECS Exec. If unset, Devsy will try to create a new role. " +
				"For more information take a look at " +
				"https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html",
		},
		"EXECUTION_ROLE_ARN": {
			Description: "ECS Execution Role ARN to use for the task definition. " +
				"If unset, Devsy will try to create a new role. For more information take a look at " +
				"https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html",
		},
		"CLUSTER_ARCHITECTURE": {
			Description: "The cpu architecture of the cluster. Can be either amd64 or arm64. Defaults to amd64",
			Default:     "amd64",
			Enum:        []string{"amd64", "arm64"},
		},
		"TASK_CPU": {
			Description: "ECS Task cpu as a string. If using Fargate, make sure the combination " +
				"with TASK_MEMORY is supported. E.g. '.5 vcpu'. Learn more at " +
				"https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#task_size",
			Default: "2 vcpu",
		},
		"TASK_MEMORY": {
			Description: "ECS Task memory as a string. If using Fargate, make sure the combination " +
				"with TASK_CPU is supported. E.g. '1 gb'. Learn more at " +
				"https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#task_size",
			Default: "4 gb",
		},
		"LAUNCH_TYPE": {
			Description: "ECS Task Launch Type, which can be either FARGATE, ECS or EXTERNAL",
			Default:     "FARGATE",
			Enum:        []string{"FARGATE", "EC2", "EXTERNAL"},
		},
		"ASSIGN_PUBLIC_IP": {
			Description: "If the task should get a public ip assigned. " +
				"For public subnets specify ENABLED and for private subnets specify DISABLED.",
			Default: "ENABLED",
			Enum:    []string{"ENABLED", "DISABLED"},
		},
		"SECURITY_GROUP_ID": {
			Description: "ECS Security Group ID to attach to the network settings of the ECS task.",
		},
	}
}

func buildAgent(cfg *buildConfig) Agent {
	return Agent{
		ContainerInactivityTimeout: "${INACTIVITY_TIMEOUT}",
		Local:                      true,
		Binaries: map[string]any{
			"ECS_PROVIDER": buildBinaryList(cfg, allPlatforms()),
		},
		DockerlessIgnorePaths: "/managed-agents",
		Dockerless: Dockerless{
			IgnorePaths: "/managed-agents",
		},
		Driver: "custom",
		Custom: CustomDriver{
			FindDevContainer:    "${ECS_PROVIDER} find",
			CommandDevContainer: "${ECS_PROVIDER} command",
			StartDevContainer:   "${ECS_PROVIDER} start",
			StopDevContainer:    "${ECS_PROVIDER} stop",
			RunDevContainer:     "${ECS_PROVIDER} run",
			DeleteDevContainer:  "${ECS_PROVIDER} delete",
			TargetArchitecture:  "${ECS_PROVIDER} target-architecture",
		},
	}
}

func buildBinaryList(cfg *buildConfig, platforms []string) []Binary {
	result := make([]Binary, 0, len(platforms))
	for _, platform := range platforms {
		result = append(result, buildBinary(cfg, platform))
	}
	return result
}

func buildBinary(cfg *buildConfig, platform string) Binary {
	os, arch, _ := strings.Cut(platform, "/")

	path := cfg.projectRoot
	if !cfg.isRelease {
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			base, _ := url.Parse(path)
			joined, _ := url.JoinPath(base.String(), buildDir(platform))
			path = joined
		} else {
			absPath, _ := filepath.Abs(path)
			path = filepath.Join(absPath, buildDir(platform))
		}
	}

	filename := fmt.Sprintf("devsy-provider-%s-%s-%s", providerName, os, arch)
	if os == "windows" {
		filename += ".exe"
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		path, _ = url.JoinPath(path, filename)
	} else {
		path = filepath.Join(path, filename)
	}

	return Binary{
		OS:       os,
		Arch:     arch,
		Path:     path,
		Checksum: cfg.checksums[filename],
	}
}

func buildDir(platform string) string {
	dirs := map[string]string{
		"linux/amd64":   "build_linux_amd64_v1",
		"linux/arm64":   "build_linux_arm64_v8.0",
		"darwin/amd64":  "build_darwin_amd64_v1",
		"darwin/arm64":  "build_darwin_arm64_v8.0",
		"windows/amd64": "build_windows_amd64_v1",
	}
	return dirs[platform]
}

func allPlatforms() []string {
	return []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64"}
}

func parseChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path) //nolint:gosec // path is a build-time constant, not user input
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if checksum, filename, ok := strings.Cut(scanner.Text(), "  "); ok {
			checksums[strings.TrimSpace(filename)] = checksum
		}
	}

	return checksums, scanner.Err()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

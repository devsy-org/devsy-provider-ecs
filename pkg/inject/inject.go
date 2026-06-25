package inject

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/devsy-org/devsy-provider-ecs/pkg/version"
)

//go:embed inject.sh
var Script string

const releaseBaseURL = "https://github.com/devsy-org/devsy-provider-ecs/releases"

const BaseURL = releaseBaseURL + "/download/%s/devsy-provider-ecs-linux-%s"

const LatestBaseURL = releaseBaseURL + "/latest/download/devsy-provider-ecs-linux-%s"

func GetContainerEntrypoint(entrypoint []string, cmd []string) ([]string, []string, error) {
	downloadAmd := ""
	downloadArm := ""
	if version.Version == "latest" {
		downloadAmd = fmt.Sprintf(LatestBaseURL, "amd64")
		downloadArm = fmt.Sprintf(LatestBaseURL, "arm64")
	} else {
		downloadAmd = fmt.Sprintf(BaseURL, version.Version, "amd64")
		downloadArm = fmt.Sprintf(BaseURL, version.Version, "arm64")
	}

	command := "/workspaces/devsy-provider-ecs entrypoint"
	if len(entrypoint) > 0 {
		out, err := json.Marshal(entrypoint)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal entrypoint: %w", err)
		}

		command += fmt.Sprintf(" --entrypoint '%s'", base64.StdEncoding.EncodeToString(out))
	}
	if len(cmd) > 0 {
		out, err := json.Marshal(cmd)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal cmd: %w", err)
		}

		command += fmt.Sprintf(" --cmd '%s'", base64.StdEncoding.EncodeToString(out))
	}

	injectScript, err := FillTemplate(Script, map[string]string{
		"DownloadAmd":     downloadAmd,
		"DownloadArm":     downloadArm,
		"InstallFilename": "devsy-provider-ecs",
		"InstallDir":      "/workspaces",
		"Command":         command,
	})
	if err != nil {
		return nil, nil, err
	}

	return []string{"sh"}, []string{"-c", injectScript}, nil
}

func FillTemplate(templateString string, vars any) (string, error) {
	t, err := template.New("gotmpl").Parse(templateString)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	err = t.Execute(&buf, vars)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

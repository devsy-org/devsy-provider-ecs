package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
)

const (
	infoCanary = "info canary"
)

func TestProviderLogChild(_ *testing.T) {
	if os.Getenv("ECS_LOG_TEST_CHILD") != "1" {
		return
	}
	cmd := NewRootCmd()
	cmd.AddCommand(&cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) {
		log.Debug("debug canary")
		log.Info("redaction canary ", os.Getenv("ECS_TEST_SECRET"))
		log.Info(infoCanary)
		log.Warn("warn canary")
	}})
	cmd.SetArgs([]string{"probe"})
	if err := cmd.Execute(); err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
		os.Exit(2)
	}
}

func TestProviderLogging(t *testing.T) {
	for _, tc := range []struct {
		name    string
		level   string
		format  string
		want    string
		absent  string
		wantErr bool
	}{
		{name: "default", want: infoCanary, absent: "debug canary"},
		{name: "warn", level: "warn", want: "warn canary", absent: infoCanary},
		{name: "uppercase warn", level: "WARN", want: "warn canary", absent: infoCanary},
		{name: "trace", level: "trace", want: "debug canary"},
		{name: "auto", format: "auto", want: infoCanary},
		{name: "uppercase auto", format: "AUTO", want: infoCanary},
		{name: "whitespace rejected", level: " info ", wantErr: true},
		{name: "invalid level", level: "secret-level-canary", absent: "secret-level-canary", wantErr: true},
		{name: "invalid format", format: "secret-format-canary", absent: "secret-format-canary", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := providerLoggingChild(t)
			child.Env = append(
				os.Environ(),
				"ECS_LOG_TEST_CHILD=1",
				"ECS_TEST_SECRET=private-secret-value-8392",
				"DEVSY_LOG_LEVEL="+tc.level,
				"DEVSY_LOG_FORMAT="+tc.format,
			)
			output, err := child.CombinedOutput()
			if (err != nil) != tc.wantErr {
				t.Fatalf("child error = %v, output = %s", err, output)
			}
			assertExpectedDiagnostics(t, string(output), tc.want, tc.absent)
			if strings.Contains(string(output), "private-secret-value-8392") {
				t.Fatalf("secret leaked in %s", output)
			}
			if !tc.wantErr && !strings.EqualFold(tc.level, "warn") &&
				!strings.Contains(string(output), "redaction canary ***") {
				t.Fatalf("missing redacted diagnostic in %s", output)
			}
			assertProviderDiagnosticOutput(t, string(output), tc.wantErr)
		})
	}
}

func TestProviderLogFormats(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "TEXT", want: "text"},
		{input: "PLAIN", want: "text"},
		{input: "LOGFMT", want: "logfmt"},
		{input: "JSON", want: "json"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", "")
			t.Setenv("DEVSY_LOG_FORMAT", tc.input)
			cfg, err := providerLogConfig()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Format != tc.want || cfg.Level != log.LevelInfoName {
				t.Fatalf("unexpected provider logging configuration: %#v", cfg)
			}
		})
	}
}

func assertProviderDiagnosticOutput(t *testing.T, output string, wantErr bool) {
	t.Helper()
	if wantErr {
		if strings.Contains(output, "canary\n") {
			t.Fatalf("command executed with invalid configuration: %s", output)
		}
		return
	}
	assertProviderLogEnvelope(t, output)
}

func assertProviderLogEnvelope(t *testing.T, output string) {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if line == "PASS" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid JSON record %q: %v", line, err)
		}
		if record["ts"] == nil || record["level"] == nil || record["msg"] == nil {
			t.Fatalf("missing log envelope fields: %v", record)
		}
	}
}

func providerLoggingChild(t *testing.T) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &exec.Cmd{
		Path: executable,
		Args: []string{executable, "-test.run=^TestProviderLogChild$"},
	}
}

func assertExpectedDiagnostics(t *testing.T, output, want, absent string) {
	t.Helper()
	if want != "" && !strings.Contains(output, want) {
		t.Fatalf("missing %q in %s", want, output)
	}
	if absent != "" && strings.Contains(output, absent) {
		t.Fatalf("unexpected %q in %s", absent, output)
	}
}

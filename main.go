package main

import "github.com/devsy-org/devsy-provider-ecs/cmd"

// Injected at build time via goreleaser ldflags.
var (
	build  = "latest"
	commit = ""
	date   = ""
)

func main() {
	cmd.Execute(cmd.BuildInfo{Version: build, Commit: commit, Date: date})
}

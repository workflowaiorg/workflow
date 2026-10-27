// Command wf brings an AI-assisted engineering workflow into a Go project.
package main

import (
	"os"

	"github.com/workflowaiorg/workflow/internal/cli"
)

// version is set by the release build with -ldflags "-X main.version=…". Any other build has
// none, and takes its version from the module Go recorded instead.
var version string

func main() {
	os.Exit(cli.New(cli.ResolveVersion(version), os.Stdout, os.Stderr).Run(os.Args[1:]))
}

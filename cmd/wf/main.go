// Command wf brings an AI-assisted engineering workflow into a Go project.
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/workflowaiorg/workflow/internal/cli"
)

// version is set by the release build with -ldflags "-X main.version=…". Any other build has
// none, and takes its version from the module Go recorded instead.
var version string

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "wf: finding the current directory: %v\n", err)
		os.Exit(1)
	}
	os.Exit(cli.Main(cli.ResolveVersion(version), dir, os.Args[1:], os.Environ(), os.Stdin, os.Stdout, os.Stderr, syscall.Exec))
}

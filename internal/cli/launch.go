package cli

import (
	"fmt"
	"io"

	"github.com/workflowaiorg/workflow/internal/launcher"
)

// Exec replaces the running process with binary, run with argv and env. It returns only when it
// could not.
type Exec func(binary string, argv, env []string) error

// configureLauncher adjusts the launcher Main makes; the tests point it at a fake release server.
var configureLauncher = func(*launcher.Launcher) {}

// Main runs wf as version in the project at dir, and returns the exit code. When the project pins
// another version, it runs that one in its place through exec instead, and returns only if it
// could not.
func Main(version, dir string, args, environ []string, stdin io.Reader, stdout, stderr io.Writer, exec Exec) int {
	// A wf a launcher started runs itself; any other first runs the version the project pins.
	handedOver, err := launcher.Handover(version)
	if err != nil {
		fmt.Fprintf(stderr, "wf: %v\n", err)
		return exitCode(err)
	}
	if !handedOver {
		l := launcher.New(version, dir)
		configureLauncher(l)
		target, err := l.Resolve(args)
		if err != nil {
			fmt.Fprintf(stderr, "wf: %v\n", err)
			return exitCode(err)
		}
		if target.Binary != "" {
			// The pinned version's arguments, terminal, signals and exit code are its own.
			err := exec(target.Binary, append([]string{target.Binary}, args...), launcher.Environ(environ, target))
			fmt.Fprintf(stderr, "wf: running wf %s from %s: %v; if it keeps failing, delete that folder\n", target.Version, target.Binary, err)
			return exitFailure
		}
	}
	return New(version, dir, stdin, stdout, stderr).Run(args)
}

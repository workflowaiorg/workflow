// Package stack runs a project's local stack: docker compose on the project's compose file, and
// the doctor checks that say whether the machine and the stack are ready.
package stack

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/workflowaiorg/workflow/internal/project"
)

// Stack is the stack of the project at Dir, writing what it runs to Stdout and Stderr.
type Stack struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Up starts the stack in the background, or with services named, starts those and stays attached
// to their logs until they stop or the developer presses Ctrl-C.
func (s Stack) Up(services ...string) error {
	if len(services) == 0 {
		return s.compose("up", "-d")
	}
	// Ctrl-C reaches docker compose too, which stops the services and then exits: wf waits for it
	// rather than return to the prompt while it is still stopping them.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	err := s.compose(append([]string{"up"}, services...)...)
	if len(interrupts) > 0 {
		return ErrInterrupted
	}
	return err
}

// ErrInterrupted is a command the developer stopped with Ctrl-C: not a failure to report.
var ErrInterrupted = errors.New("interrupted")

// Down stops the stack and removes its containers.
func (s Stack) Down() error {
	return s.compose("down")
}

func (s Stack) compose(args ...string) error {
	command := exec.Command("docker", append([]string{"compose"}, args...)...)
	command.Dir = s.Dir
	command.Stdin, command.Stdout, command.Stderr = s.Stdin, s.Stdout, s.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// isUp says whether any service of the stack is running.
func (s Stack) isUp() bool {
	command := exec.Command("docker", "compose", "ps", "--status", "running", "--quiet")
	command.Dir = s.Dir
	out, err := command.Output()
	return err == nil && len(bytes.TrimSpace(out)) > 0
}

// shell runs line with sh in the project, its output going where s writes.
func (s Stack) shell(line string) error {
	command := exec.Command("sh", "-c", line)
	command.Dir = s.Dir
	command.Stdout, command.Stderr = s.Stdout, s.Stderr
	return command.Run()
}

// probe runs a check's command with sh in the project, and returns whether it passed and what it
// printed. A command that cannot be started at all is an error, not a failed check.
func (s Stack) probe(check project.Check) (bool, string, error) {
	command := exec.Command("sh", "-c", check.Run)
	command.Dir = s.Dir
	var out bytes.Buffer
	command.Stdout, command.Stderr = &out, &out
	err := command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, out.String(), nil
	case errors.As(err, &exit):
		return false, out.String(), nil
	default:
		return false, "", fmt.Errorf("running the check %q: %w", check.Name, err)
	}
}

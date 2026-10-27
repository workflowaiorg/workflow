// Package cli is wf's command line: the commands it knows, and how they are found and run.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
)

// Exit codes. Usage errors are told apart from failures, so a script can tell a typo from a fault.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// errUsage marks an error that is the caller's mistake, not wf's.
var errUsage = errors.New("usage")

// Command is one wf command.
type Command struct {
	Name    string `json:"name"`
	Usage   string `json:"usage"`
	Summary string `json:"summary"`
	run     func(app *App, args []string) error
}

// App holds what every command needs: the version wf reports and where it writes.
type App struct {
	version  string
	stdout   io.Writer
	stderr   io.Writer
	commands []Command
}

// New returns wf's command line, reporting version and writing to stdout and stderr.
func New(version string, stdout, stderr io.Writer) *App {
	app := &App{version: version, stdout: stdout, stderr: stderr}
	app.commands = []Command{
		{Name: "version", Usage: "wf version", Summary: "Print wf's version", run: runVersion},
		{Name: "help", Usage: "wf help [--json]", Summary: "List wf's commands; --json for agents", run: runHelp},
	}
	return app
}

// Run runs the command args names and returns the process exit code.
func (app *App) Run(args []string) int {
	if len(args) == 0 {
		args = []string{"help"}
	}
	name, rest := args[0], args[1:]
	if name == "-h" || name == "--help" {
		name = "help"
	}

	index := slices.IndexFunc(app.commands, func(command Command) bool { return command.Name == name })
	if index < 0 {
		fmt.Fprintf(app.stderr, "wf: unknown command %q; run wf help\n", name)
		return exitUsage
	}

	if err := app.commands[index].run(app, rest); err != nil {
		fmt.Fprintf(app.stderr, "wf %s: %v\n", name, err)
		if errors.Is(err, errUsage) {
			return exitUsage
		}
		return exitFailure
	}
	return exitOK
}

func runVersion(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: takes no arguments", errUsage)
	}
	_, err := fmt.Fprintln(app.stdout, app.version)
	return err
}

func runHelp(app *App, args []string) error {
	switch strings.Join(args, " ") {
	case "":
		return app.writeHelp()
	case "--json":
		return app.writeHelpJSON()
	default:
		return fmt.Errorf("%w: only --json is accepted", errUsage)
	}
}

func (app *App) writeHelp() error {
	fmt.Fprintf(app.stdout, "wf %s — an AI-assisted engineering workflow for Go projects\n\nCommands:\n", app.version)
	table := tabwriter.NewWriter(app.stdout, 0, 0, 3, ' ', 0)
	for _, command := range app.commands {
		fmt.Fprintf(table, "  %s\t%s\n", command.Usage, command.Summary)
	}
	return table.Flush()
}

// helpJSON is what wf help --json prints: the contract agents read, so its shape only grows.
type helpJSON struct {
	Version  string    `json:"version"`
	Commands []Command `json:"commands"`
}

func (app *App) writeHelpJSON() error {
	encoder := json.NewEncoder(app.stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(helpJSON{Version: app.version, Commands: app.commands}); err != nil {
		return fmt.Errorf("encoding the command list: %w", err)
	}
	return nil
}

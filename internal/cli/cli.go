// Package cli is wf's command line: the commands it knows, and how they are found and run.
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/component/backend"
	"github.com/workflowaiorg/workflow/internal/component/frontend"
	"github.com/workflowaiorg/workflow/internal/launcher"
	"github.com/workflowaiorg/workflow/internal/project"
)

// Exit codes. Usage errors are told apart from failures, so a script can tell a typo from a fault.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// errUsage marks an error that is the caller's mistake, not wf's.
var errUsage = errors.New("usage")

// errHelpShown ends a command whose -h has printed its usage.
var errHelpShown = errors.New("help shown")

// Command is one wf command.
type Command struct {
	Name    string `json:"name"`
	Usage   string `json:"usage"`
	Summary string `json:"summary"`
	run     func(app *App, args []string) error
}

// App holds what every command needs: the version wf reports, the project it works on, where it
// reads answers from, and where it writes.
type App struct {
	version string
	dir     string
	stdin   *bufio.Reader
	// interactive is whether stdin is a terminal a person answers questions on.
	interactive bool
	stdout      io.Writer
	stderr      io.Writer
	commands    []Command
	// components are the components wf add offers.
	components []component.Component
	now        func() time.Time
	// releases is where wf lock reads a release's checksums from.
	releases string
	// exec hands the command over to another version of wf, with environ; nil when this wf cannot.
	exec    Exec
	environ []string
}

// New returns wf's command line, reporting version, working on the project at dir, reading answers
// from stdin and writing to stdout and stderr. wf asks a question only when stdin is a terminal.
func New(version, dir string, stdin io.Reader, stdout, stderr io.Writer) *App {
	app := &App{version: version, dir: dir, stdin: bufio.NewReader(stdin), interactive: isTerminal(stdin), stdout: stdout, stderr: stderr, components: []component.Component{backend.Component, frontend.Component}, now: time.Now, releases: launcher.ReleasesURL}
	app.commands = []Command{
		{Name: "init", Usage: "wf init [--forge <forge>]", Summary: "Start using wf in this project: .wf.yml, the base, and every assistant's files", run: runInit},
		{Name: "add", Usage: "wf add [<component>] [--<option> …] [--force]", Summary: "Add a component to the project; with no component, list them", run: runAdd},
		{Name: "lock", Usage: "wf lock", Summary: "Record the pinned wf release's fingerprints in .wf.sum, which every download of it is checked against", run: runLock},
		{Name: "update", Usage: "wf update [--to <version>]", Summary: "Bring in the pinned wf's base and sync; --to pins that version first", run: runUpdate},
		{Name: "sync", Usage: "wf sync [--check | --force]", Summary: "Write every assistant's files from .agents; --check only compares", run: runSync},
		{Name: "show", Usage: "wf show <skill> [--base]", Summary: "Print a skill as an assistant reads it; --base for wf's part alone", run: runShow},
		{Name: "worktree", Usage: "wf worktree <issue> <slug> | wf worktree plan <slug>", Summary: "Give an Issue or a plan its own worktree, or find it; prints its path last", run: runWorktree},
		{Name: "doctor", Usage: "wf doctor", Summary: "Check that the machine and the repo are ready; changes nothing", run: runDoctor},
		{Name: "scaffold", Usage: "wf scaffold", Summary: "Take a clean clone to a ready repo: run the fix of each check that fails", run: runScaffold},
		{Name: "spec-coverage", Usage: "wf spec-coverage [--report-only]", Summary: "Check that specs, decision records and their index agree", run: runSpecCoverage},
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
	run := app.commands[index].run

	if err := run(app, rest); err != nil {
		if errors.Is(err, errHelpShown) {
			return exitOK
		}
		fmt.Fprintf(app.stderr, "wf %s: %v\n", name, err)
		return exitCode(err)
	}
	return exitOK
}

// exitCode is the exit code of a command that failed with err: the developer's mistake, or wf's
// failure.
func exitCode(err error) int {
	var mistake *project.UserError
	if errors.Is(err, errUsage) || errors.As(err, &mistake) {
		return exitUsage
	}
	return exitFailure
}

// parseFlags parses args into flags, and any mistake in them is a usage error. -h prints the
// command's usage line and ends it.
func (app *App) parseFlags(flags *flag.FlagSet, args []string) error {
	flags.SetOutput(io.Discard)
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		index := slices.IndexFunc(app.commands, func(command Command) bool { return command.Name == flags.Name() })
		fmt.Fprintf(app.stdout, "usage: %s\n%s\n", app.commands[index].Usage, app.commands[index].Summary)
		return errHelpShown
	}
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	return nil
}

// ask returns the answer to question, or fallback when it is left empty. With no terminal to ask
// on, it fails naming the flag that gives the answer instead, rather than wait for an answer
// nobody will type: an agent has no terminal, so it asks the developer and passes the flag.
func (app *App) ask(question, fallback, flags string) (string, error) {
	if !app.interactive {
		return "", fmt.Errorf("%w: no terminal to ask %q on; pass %s", errUsage, question, flags)
	}
	fmt.Fprintf(app.stdout, "%s [%s]: ", question, fallback)
	answer, err := app.stdin.ReadString('\n')
	if errors.Is(err, io.EOF) && answer == "" {
		return "", fmt.Errorf("%w: no answer to %q; pass %s", errUsage, question, flags)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading the answer to %q: %w", question, err)
	}
	if answer = strings.TrimSpace(answer); answer != "" {
		return answer, nil
	}
	return fallback, nil
}

// isTerminal says whether stdin is a terminal rather than a pipe, a file or /dev/null.
func isTerminal(stdin io.Reader) bool {
	file, ok := stdin.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
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

// load reads the settings of the project in app.dir, and the components they install.
func (app *App) load() (project.Config, []component.Component, error) {
	config, err := project.Load(app.dir)
	if err != nil {
		return project.Config{}, nil, err
	}
	installed, err := component.Installed(config, app.components)
	if err != nil {
		return project.Config{}, nil, err
	}
	return config, installed, nil
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

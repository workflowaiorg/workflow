package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/render"
	"github.com/workflowaiorg/workflow/internal/specs"
	"github.com/workflowaiorg/workflow/internal/worktree"
)

// configTemplate is the .wf.yml wf init writes. The commented keys are there to be found.
const configTemplate = `# wf's settings for this project. wf reads them; people edit them.
wf: %s
forge: %s

# Gitignored files wf worktree copies into a new worktree, as globs from the project's root.
# worktree:
#   copy: [".env*"]
#   skip: ["*.example"]

# The project's own gate steps for /pre-mr-check, each run when a changed path matches.
# gate:
#   - name: lint
#     paths: ["**/*.go"]
#     run: golangci-lint run ./...
`

func runInit(app *App, args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	forge := flags.String("forge", "", "the forge the project's Issues and merge requests live on: "+strings.Join(base.Forges, ", "))
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments besides --forge", errUsage)
	}
	if _, err := os.Lstat(filepath.Join(app.dir, render.ConfigFile)); err == nil {
		return fmt.Errorf("%w: this project already has %s", errUsage, render.ConfigFile)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", render.ConfigFile, err)
	}

	if *forge == "" {
		answer, err := app.ask("Which forge do this project's Issues live on ("+strings.Join(base.Forges, ", ")+")", base.Forges[0], "forge")
		if err != nil {
			return err
		}
		*forge = answer
	}
	if !slices.Contains(base.Forges, *forge) {
		return fmt.Errorf("%w: wf works with %s for now, not %q", errUsage, strings.Join(base.Forges, ", "), *forge)
	}

	if err := base.Install(app.dir, *forge); err != nil {
		return err
	}
	written, err := base.Skeleton(app.dir)
	if err != nil {
		return err
	}
	fmt.Fprintln(app.stdout, "wrote    "+render.BaseDir+"/")
	for _, path := range written {
		fmt.Fprintln(app.stdout, "wrote    "+path+" (the project's from now on)")
	}
	// .wf.yml comes last of what init writes itself, so an init that failed before it can be run again.
	if err := os.WriteFile(filepath.Join(app.dir, render.ConfigFile), fmt.Appendf(nil, configTemplate, app.version, *forge), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", render.ConfigFile, err)
	}
	fmt.Fprintln(app.stdout, "wrote    "+render.ConfigFile)

	report, err := render.Sync(app.dir, app.version, false)
	return app.reportSync(report, err)
}

var (
	issueNumber = regexp.MustCompile(`^[1-9][0-9]*$`)
	kebabSlug   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

func runWorktree(app *App, args []string) error {
	flags := flag.NewFlagSet("worktree", flag.ContinueOnError)
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return fmt.Errorf("%w: names an Issue and a slug, or plan and a slug", errUsage)
	}
	issue, slug := flags.Arg(0), flags.Arg(1)
	if issue != "plan" && !issueNumber.MatchString(issue) {
		return fmt.Errorf("%w: %q is neither an Issue number nor plan", errUsage, issue)
	}
	if !kebabSlug.MatchString(slug) {
		return fmt.Errorf("%w: the slug %q is not short-kebab-case", errUsage, slug)
	}

	result, err := worktree.Open(app.dir, issue, slug)
	if err != nil {
		return err
	}
	for _, note := range result.Notes {
		fmt.Fprintln(app.stderr, note)
	}
	// The path comes last and alone on its line, for the agent that goes there next.
	_, err = fmt.Fprintln(app.stdout, result.Path)
	return err
}

func runSpecCoverage(app *App, args []string) error {
	flags := flag.NewFlagSet("spec-coverage", flag.ContinueOnError)
	reportOnly := flags.Bool("report-only", false, "print the report and exit 0")
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments besides --report-only", errUsage)
	}

	report, err := specs.Check(app.dir, app.now())
	if err != nil {
		return err
	}
	report.Write(app.stdout)
	if len(report.Problems) == 0 {
		return nil
	}
	if *reportOnly {
		fmt.Fprintln(app.stdout, "\nReport-only: exiting 0. The same problems fail the check.")
		return nil
	}
	if len(report.Problems) == 1 {
		return errors.New("1 problem in .workflow")
	}
	return fmt.Errorf("%d problems in .workflow", len(report.Problems))
}

package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/project"
	"github.com/workflowaiorg/workflow/internal/render"
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
	forge := flags.String("forge", "", "the forge the project's Issues and merge requests live on: "+strings.Join(project.Forges, ", "))
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments besides --forge", errUsage)
	}
	if _, err := os.Lstat(filepath.Join(app.dir, project.ConfigFile)); err == nil {
		return fmt.Errorf("%w: this project already has %s", errUsage, project.ConfigFile)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", project.ConfigFile, err)
	}

	if *forge == "" {
		answer, err := app.ask("Which forge do this project's Issues live on ("+strings.Join(project.Forges, ", ")+")", project.Forges[0], "forge")
		if err != nil {
			return err
		}
		*forge = answer
	}
	if !slices.Contains(project.Forges, *forge) {
		return fmt.Errorf("%w: wf works with %s for now, not %q", errUsage, strings.Join(project.Forges, ", "), *forge)
	}

	if err := base.Install(app.dir, *forge, nil); err != nil {
		return err
	}
	written, err := base.Skeleton(app.dir)
	if err != nil {
		return err
	}
	fmt.Fprintln(app.stdout, "wrote    "+project.BaseDir+"/")
	for _, path := range written {
		fmt.Fprintln(app.stdout, "wrote    "+path+" (the project's from now on)")
	}
	// .wf.yml comes last of what init writes itself, so an init that failed before it can be run again.
	if err := os.WriteFile(filepath.Join(app.dir, project.ConfigFile), fmt.Appendf(nil, configTemplate, app.version, *forge), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", project.ConfigFile, err)
	}
	fmt.Fprintln(app.stdout, "wrote    "+project.ConfigFile)

	config, err := project.Load(app.dir)
	if err != nil {
		return err
	}
	report, err := render.Sync(app.dir, config, nil, app.version, false)
	return app.reportSync(report, err)
}

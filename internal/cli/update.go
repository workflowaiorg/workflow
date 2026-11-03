package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/launcher"
	"github.com/workflowaiorg/workflow/internal/project"
	"github.com/workflowaiorg/workflow/internal/render"
)

func runUpdate(app *App, args []string) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	to := flags.String("to", "", "the wf version to move the pin to")
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments besides --to", errUsage)
	}
	toSet := false
	flags.Visit(func(f *flag.Flag) { toSet = toSet || f.Name == "to" })
	if toSet && *to == "" {
		return fmt.Errorf("%w: --to needs a version, such as 0.2.0", errUsage)
	}
	if toSet {
		handedOver, err := app.movePin(*to)
		if err != nil || handedOver {
			return err
		}
	}

	config, installed, err := app.load()
	if err != nil {
		return err
	}
	if config.WF != app.version {
		return project.UserErrorf("%s: pins wf %s, but this is wf %s; run the version it pins", project.ConfigFile, config.WF, app.version)
	}
	changed, err := base.Changed(app.dir, config.Forge, installed)
	if err != nil {
		return err
	}
	if err := base.Install(app.dir, config.Forge, installed); err != nil {
		return err
	}
	if len(changed) > 0 {
		fmt.Fprintf(app.stdout, "wrote    %s/ (wf %s's)\n", project.BaseDir, app.version)
	}
	// A part of the project's own may now contradict the wf part it is read with.
	for _, name := range changed {
		part := path.Join(project.ProjectDir, name)
		if _, err := os.Stat(filepath.Join(app.dir, filepath.FromSlash(part))); err == nil {
			fmt.Fprintf(app.stderr, "check    %s (wf's part of it changed; wf show prints the two together)\n", part)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	report, err := render.Sync(app.dir, config, installed, app.version, false)
	return app.reportSync(report, err)
}

// movePin pins wf version in .wf.yml and records its fingerprints in .wf.sum, as wf lock does. When
// version is not the wf running, it hands the rest of the update over to version, and returns only
// if it could not.
func (app *App) movePin(version string) (handedOver bool, err error) {
	if err := launcher.CheckVersion(version); err != nil {
		return false, err
	}
	l := app.launcher()
	// Only version can finish the update, and this wf may not hand over to it.
	if l.NoLaunch && version != app.version {
		return false, project.UserErrorf("%s=1 runs only this wf, %s; run wf update --to %s without it", launcher.NoLaunchEnv, app.version, version)
	}
	file := filepath.Join(app.dir, project.ConfigFile)
	before, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, project.UserErrorf("no %s in %s: run wf in the project's root", project.ConfigFile, app.dir)
		}
		return false, err
	}
	after, err := project.WithPin(app.dir, version)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(file, after, 0o644); err != nil {
		return false, fmt.Errorf("writing %s: %w", project.ConfigFile, err)
	}

	result, err := l.Lock()
	if err != nil {
		// A pin with no fingerprints stops every later wf command, so it is put back.
		if restoreErr := os.WriteFile(file, before, 0o644); restoreErr != nil {
			return false, errors.Join(err, fmt.Errorf("putting %s back: %w", project.ConfigFile, restoreErr))
		}
		return false, fmt.Errorf("pinning wf %s: %w; %s was left as it was", version, err, project.ConfigFile)
	}
	fmt.Fprintf(app.stdout, "wrote    %s (wf %s)\n", project.ConfigFile, version)
	if result.Changed {
		fmt.Fprintf(app.stdout, "wrote    %s (wf %s's fingerprints)\n", project.SumFile, version)
	}

	target, err := l.Resolve([]string{"update"})
	if err != nil || target.Binary == "" {
		return false, err
	}
	if app.exec == nil {
		return false, errors.New("this wf cannot hand over to another version")
	}
	err = app.exec(target.Binary, []string{target.Binary, "update"}, launcher.Environ(app.environ, target))
	return true, fmt.Errorf("running wf %s from %s: %w; if it keeps failing, delete that folder", target.Version, target.Binary, err)
}

// launcher returns the launcher of the wf running, in its project.
func (app *App) launcher() *launcher.Launcher {
	l := launcher.New(app.version, app.dir)
	l.ReleasesURL = app.releases
	configureLauncher(l)
	return l
}

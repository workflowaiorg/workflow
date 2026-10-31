package cli

import (
	"bytes"
	"time"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/launcher"
)

// ModuleVersion exposes moduleVersion to the external tests.
var ModuleVersion = moduleVersion

// SetInteractive makes app ask its questions as if stdin were a terminal.
func (app *App) SetInteractive() { app.interactive = true }

// SetNow fixes the time app reads, for checks that depend on it.
func (app *App) SetNow(now time.Time) { app.now = func() time.Time { return now } }

// Output returns the buffers app writes to, when it was made with them.
func (app *App) Output() (stdout, stderr *bytes.Buffer) {
	return app.stdout.(*bytes.Buffer), app.stderr.(*bytes.Buffer)
}

// SetComponents makes components the ones wf add offers.
func (app *App) SetComponents(components ...component.Component) { app.components = components }

// SetReleases makes wf lock read releases from url.
func (app *App) SetReleases(url string) { app.releases = url }

// ConfigureLauncher makes Main adjust its launcher with configure, until the returned func runs.
func ConfigureLauncher(configure func(*launcher.Launcher)) (restore func()) {
	previous := configureLauncher
	configureLauncher = configure
	return func() { configureLauncher = previous }
}

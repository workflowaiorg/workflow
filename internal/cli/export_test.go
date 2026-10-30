package cli

import (
	"bytes"
	"time"
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

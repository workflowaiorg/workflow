package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/cli"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runIn(t, t.TempDir(), args...)
}

// runIn runs wf on the project at dir.
func runIn(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.New("0.1.0", dir, &out, &errOut).Run(args)
	return code, out.String(), errOut.String()
}

func TestVersion(t *testing.T) {
	code, stdout, stderr := run(t, "version")
	if code != 0 || stdout != "0.1.0\n" || stderr != "" {
		t.Fatalf("wf version = (%d, %q, %q), want (0, %q, \"\")", code, stdout, stderr, "0.1.0\n")
	}
}

func TestHelpJSONListsEveryCommand(t *testing.T) {
	code, stdout, stderr := run(t, "help", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("wf help --json = (%d, stderr %q), want (0, \"\")", code, stderr)
	}

	var help struct {
		Version  string `json:"version"`
		Commands []struct {
			Name    string `json:"name"`
			Usage   string `json:"usage"`
			Summary string `json:"summary"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(stdout), &help); err != nil {
		t.Fatalf("wf help --json is not JSON: %v\n%s", err, stdout)
	}
	if help.Version != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", help.Version)
	}

	var names []string
	for _, command := range help.Commands {
		if command.Usage == "" || command.Summary == "" {
			t.Errorf("command %q has no usage or summary", command.Name)
		}
		names = append(names, command.Name)
	}
	want := []string{"sync", "show", "version", "help"}
	if !slices.Equal(names, want) {
		t.Errorf("commands = %v, want %v", names, want)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	code, stdout, stderr := run(t, "help")
	if code != 0 || stderr != "" {
		t.Fatalf("wf help = (%d, stderr %q), want (0, \"\")", code, stderr)
	}
	for _, usage := range []string{"wf sync [--check | --force]", "wf show <skill> [--base]", "wf version", "wf help [--json]"} {
		if !strings.Contains(stdout, usage) {
			t.Errorf("wf help does not list %q:\n%s", usage, stdout)
		}
	}
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "no arguments prints help", args: nil, want: 0},
		{name: "--help prints help", args: []string{"--help"}, want: 0},
		{name: "plain help", args: []string{"help"}, want: 0},
		{name: "unknown command", args: []string{"frobnicate"}, want: 2},
		{name: "version with an argument", args: []string{"version", "extra"}, want: 2},
		{name: "help with an unknown flag", args: []string{"help", "--yaml"}, want: 2},
		{name: "sync outside a project", args: []string{"sync"}, want: 2},
		{name: "sync with an unknown flag", args: []string{"sync", "--dry"}, want: 2},
		{name: "show with no skill", args: []string{"show"}, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := run(t, test.args...)
			if code != test.want {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, test.want, stderr)
			}
			if test.want != 0 && stderr == "" {
				t.Error("a failing command printed nothing to stderr")
			}
		})
	}
}

func TestResolveVersionPrefersTheInjectedOne(t *testing.T) {
	if got := cli.ResolveVersion("v1.2.3"); got != "1.2.3" {
		t.Errorf("ResolveVersion(v1.2.3) = %q, want 1.2.3", got)
	}
}

func TestModuleVersion(t *testing.T) {
	tests := map[string]string{
		"v0.1.0":      "0.1.0",
		"v0.2.0-rc.1": "0.2.0-rc.1",
		"(devel)":     "devel",
		"":            "devel",
	}
	for recorded, want := range tests {
		if got := cli.ModuleVersion(recorded); got != want {
			t.Errorf("ModuleVersion(%q) = %q, want %q", recorded, got, want)
		}
	}
}

func TestSyncThenCheck(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../render/testdata/sync/project")); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runIn(t, dir, "sync"); code != 0 {
		t.Fatalf("wf sync exited %d: %s", code, stderr)
	}
	if code, _, stderr := runIn(t, dir, "sync", "--check"); code != 0 {
		t.Fatalf("wf sync --check right after wf sync exited %d: %s", code, stderr)
	}

	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("edited by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runIn(t, dir, "sync", "--check")
	if code != 1 || !strings.Contains(stderr, "AGENTS.md") || !strings.Contains(stderr, "1 generated file differs") {
		t.Errorf("wf sync --check after a hand edit = (%d, %q), want exit 1 naming AGENTS.md", code, stderr)
	}
}

func TestSyncLeavesFilesItDidNotWriteUntilForced(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../render/testdata/sync/project")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("the project's own\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runIn(t, dir, "sync")
	if code != 1 || !strings.Contains(stderr, "skipped  AGENTS.md") || !strings.Contains(stderr, "--force") {
		t.Errorf("wf sync over a file wf did not write = (%d, %q), want exit 1 naming it and --force", code, stderr)
	}
	if code, _, stderr := runIn(t, dir, "sync", "--force"); code != 0 || !strings.Contains(stderr, "replaced AGENTS.md") {
		t.Errorf("wf sync --force = (%d, %q), want exit 0 listing AGENTS.md as replaced", code, stderr)
	}
	if code, _, _ := runIn(t, dir, "sync", "--check", "--force"); code != 2 {
		t.Errorf("wf sync --check --force exited %d, want 2", code)
	}
}

func TestCommandHelp(t *testing.T) {
	for _, name := range []string{"sync", "show"} {
		code, stdout, stderr := run(t, name, "-h")
		if code != 0 || !strings.HasPrefix(stdout, "usage: wf "+name) || stderr != "" {
			t.Errorf("wf %s -h = (%d, %q, %q), want exit 0 and its usage line", name, code, stdout, stderr)
		}
	}
}

func TestShow(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../render/testdata/sync/project")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"show", "hello", "--base"}, {"show", "--base", "hello"}} {
		code, stdout, stderr := runIn(t, dir, args...)
		if code != 0 || !strings.Contains(stdout, "Greets sample's developers.") || strings.Contains(stdout, "English only") {
			t.Errorf("wf %v = (%d, %q, %q), want the rendered base alone", args, code, stdout, stderr)
		}
	}
	if code, _, _ := runIn(t, dir, "show", "local", "--base"); code != 2 {
		t.Errorf("wf show local --base on a project-only skill exited %d, want 2", code)
	}
	if code, _, stderr := runIn(t, dir, "show", "absent"); code != 2 || !strings.Contains(stderr, `"absent"`) {
		t.Errorf("wf show absent = (%d, %q), want exit 2 naming the skill", code, stderr)
	}
}

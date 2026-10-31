package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/workflowaiorg/workflow/internal/cli"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runIn(t, t.TempDir(), args...)
}

// runIn runs wf on the project at dir.
func runIn(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runWith(t, cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer)), args...)
}

// runWith runs app, which writes to the buffers it was made with.
func runWith(t *testing.T, app *cli.App, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	out, errOut := app.Output()
	code = app.Run(args)
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
	want := []string{"init", "add", "lock", "sync", "show", "worktree", "up", "down", "doctor", "scaffold", "spec-coverage", "version", "help"}
	if !slices.Equal(names, want) {
		t.Errorf("commands = %v, want %v", names, want)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	code, stdout, stderr := run(t, "help")
	if code != 0 || stderr != "" {
		t.Fatalf("wf help = (%d, stderr %q), want (0, \"\")", code, stderr)
	}
	for _, usage := range []string{"wf init [--forge <forge>]", "wf add [<component>]", "wf lock", "wf sync [--check | --force]", "wf show <skill> [--base]", "wf worktree <issue> <slug>", "wf spec-coverage [--report-only]", "wf version", "wf help [--json]"} {
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
		{name: "init with an unknown forge", args: []string{"init", "--forge=sourcehut"}, want: 2},
		{name: "worktree with no slug", args: []string{"worktree", "12"}, want: 2},
		{name: "worktree with no Issue number", args: []string{"worktree", "twelve", "sound"}, want: 2},
		{name: "worktree with a slug that is not kebab-case", args: []string{"worktree", "12", "Sound_Toggle"}, want: 2},
		{name: "spec-coverage with an argument", args: []string{"spec-coverage", "specs"}, want: 2},
		{name: "spec-coverage in a project with none", args: []string{"spec-coverage"}, want: 0},
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
	if code, _, stderr := runIn(t, dir, "init", "--forge=gitlab"); code != 0 {
		t.Fatalf("wf init exited %d: %s", code, stderr)
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
	if code != 1 || !strings.Contains(stderr, "AGENTS.md") || !strings.Contains(stderr, "1 file differs") {
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
	for _, name := range []string{"init", "add", "lock", "sync", "show", "worktree", "up", "down", "doctor", "scaffold", "spec-coverage"} {
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

func TestInitThenCheckPasses(t *testing.T) {
	dir := t.TempDir()
	if code, stdout, stderr := runIn(t, dir, "init", "--forge=gitlab"); code != 0 || !strings.Contains(stdout, "wrote    .claude/skills/workflow/SKILL.md") || !strings.HasSuffix(stdout, "Next: run wf lock to record wf 0.1.0's fingerprints in .wf.sum, then commit everything.\n") {
		t.Fatalf("wf init --forge=gitlab = (%d, %q, %q), want exit 0, the skills written and wf lock named as the next step", code, stdout, stderr)
	}
	if code, _, stderr := runIn(t, dir, "sync", "--check"); code != 0 {
		t.Fatalf("wf sync --check right after wf init exited %d: %s", code, stderr)
	}
	if code, _, stderr := runIn(t, dir, "spec-coverage"); code != 0 {
		t.Errorf("wf spec-coverage right after wf init exited %d: %s", code, stderr)
	}
	if code, _, stderr := runIn(t, dir, "init", "--forge=gitlab"); code != 2 || !strings.Contains(stderr, ".wf.yml") {
		t.Errorf("a second wf init = (%d, %q), want exit 2 naming .wf.yml", code, stderr)
	}
}

func TestCheckNamesAHandEditedBaseFile(t *testing.T) {
	dir := t.TempDir()
	if code, _, stderr := runIn(t, dir, "init", "--forge=gitlab"); code != 0 {
		t.Fatalf("wf init exited %d: %s", code, stderr)
	}
	// Edited, then synced: every generated file matches the edit, so only the base check sees it.
	skill := filepath.Join(dir, ".agents/base/skills/workflow/SKILL.md")
	content, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, append(content, "One more rule.\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runIn(t, dir, "sync"); code != 0 {
		t.Fatalf("wf sync exited %d: %s", code, stderr)
	}
	code, _, stderr := runIn(t, dir, "sync", "--check")
	if code != 1 || !strings.Contains(stderr, ".agents/base/skills/workflow/SKILL.md: differs") || !strings.Contains(stderr, "1 file differs") {
		t.Errorf("wf sync --check after a hand edit to the base = (%d, %q), want exit 1 naming the file", code, stderr)
	}
}

func TestInitAsksWhatNoFlagSays(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
		input       string
		want        int
		wantStderr  string
	}{
		{name: "no terminal names the flag", input: "", want: 2, wantStderr: "pass --forge"},
		{name: "an empty answer takes the default", interactive: true, input: "\n", want: 0},
		{name: "a typed answer", interactive: true, input: "gitlab\n", want: 0},
		{name: "a forge wf has no base for", interactive: true, input: "sourcehut\n", want: 2, wantStderr: `"sourcehut"`},
		{name: "no answer at all names the flag", interactive: true, input: "", want: 2, wantStderr: "pass --forge"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			app := cli.New("0.1.0", dir, strings.NewReader(test.input), new(bytes.Buffer), new(bytes.Buffer))
			if test.interactive {
				app.SetInteractive()
			}
			code, _, stderr := runWith(t, app, "init")
			if code != test.want || !strings.Contains(stderr, test.wantStderr) {
				t.Errorf("wf init = (%d, %q), want exit %d and stderr holding %q", code, stderr, test.want, test.wantStderr)
			}
			_, err := os.Stat(filepath.Join(dir, ".wf.yml"))
			if (err == nil) != (test.want == 0) {
				t.Errorf("after exit %d, .wf.yml exists = %v", code, err == nil)
			}
		})
	}
}

func TestInitLeavesAProjectsOwnFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("the project's own\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runIn(t, dir, "init", "--forge=gitlab")
	if code != 1 || !strings.Contains(stderr, "skipped  AGENTS.md") {
		t.Errorf("wf init over the project's AGENTS.md = (%d, %q), want exit 1 listing it as skipped", code, stderr)
	}
	if content, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md")); string(content) != "the project's own\n" {
		t.Errorf("AGENTS.md was overwritten: %q", content)
	}
}

func TestPreMRCheckListsTheProjectsGateSteps(t *testing.T) {
	dir := t.TempDir()
	if code, _, stderr := runIn(t, dir, "init", "--forge=gitlab"); code != 0 {
		t.Fatalf("wf init exited %d: %s", code, stderr)
	}
	gate := "gate:\n  - name: lint\n    paths: [\"**/*.go\", go.mod]\n    run: golangci-lint run ./...\n"
	appendFile(t, filepath.Join(dir, ".wf.yml"), gate)
	if code, _, stderr := runIn(t, dir, "sync"); code != 0 {
		t.Fatalf("wf sync exited %d: %s", code, stderr)
	}
	content, err := os.ReadFile(filepath.Join(dir, ".claude/commands/pre-mr-check.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{
		"| generated files | `.wf.yml`, `.agents/**`, or any file wf generates | `wf sync --check` |",
		"| lint | `**/*.go`, `go.mod` | `golangci-lint run ./...` |",
	} {
		if !strings.Contains(string(content), row+"\n") {
			t.Errorf("/pre-mr-check has no row %q:\n%s", row, content)
		}
	}
}

func TestABadSettingIsNamedBeforeAnythingIsWritten(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want string
	}{
		{name: "no version pin", from: "wf: ", to: "# wf: ", want: "pins no wf version"},
		{name: "no forge", from: "forge: gitlab\n", to: "", want: "add forge: gitlab"},
		{name: "an unknown forge", from: "forge: gitlab", to: "forge: github", want: "forge: github is not one wf has a base for"},
		{name: "a gate step with no paths", from: "# gate:", to: "gate:\n  - name: lint\n    run: make lint\n#", want: "gate[0] (lint) needs paths:"},
		{name: "a gate step whose paths is one string", from: "# gate:", to: "gate:\n  - name: lint\n    paths: \"*.go\"\n    run: make lint\n#", want: "gate[0] (lint) needs paths:"},
		{name: "a gate step with no name or run", from: "# gate:", to: "gate:\n  - paths: [\"*.go\"]\n#", want: "gate[0] needs name: the step's name, run: the command"},
		{name: "a gate that is not a list", from: "# gate:", to: "gate: make lint\n#", want: "gate: is a list of steps"},
		{name: "a worktree glob that is no pattern", from: "# worktree:", to: "worktree:\n  copy: [\"[\"]\n#", want: `worktree.copy has a bad pattern "["`},
		{name: "a worktree copy that is one string", from: "# worktree:", to: "worktree:\n  copy: .env\n#", want: "worktree.copy is a list of globs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if code, _, stderr := runIn(t, dir, "init", "--forge=gitlab"); code != 0 {
				t.Fatalf("wf init exited %d: %s", code, stderr)
			}
			config := filepath.Join(dir, ".wf.yml")
			content, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), test.from) {
				t.Fatalf(".wf.yml has no %q to replace", test.from)
			}
			if err := os.WriteFile(config, []byte(strings.Replace(string(content), test.from, test.to, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			skill := filepath.Join(dir, ".claude", "skills", "workflow", "SKILL.md")
			before, err := os.ReadFile(skill)
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"sync"}, {"sync", "--check"}, {"show", "workflow"}} {
				if code, _, stderr := runIn(t, dir, args...); code != 2 || !strings.Contains(stderr, test.want) {
					t.Errorf("wf %s = (%d, %q), want exit 2 saying %q", strings.Join(args, " "), code, stderr, test.want)
				}
			}
			if after, err := os.ReadFile(skill); err != nil || !bytes.Equal(after, before) {
				t.Errorf("wf sync rewrote %s despite the bad setting (%v)", skill, err)
			}
		})
	}
}

func TestSpecCoverageFailsOnAProblemUnlessReportOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../specs/testdata/valid")); err != nil {
		t.Fatal(err)
	}
	app := func() *cli.App {
		app := cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
		app.SetNow(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
		return app
	}
	if code, stdout, stderr := runWith(t, app(), "spec-coverage"); code != 0 || !strings.Contains(stdout, "plan debt   0") {
		t.Fatalf("wf spec-coverage on the valid sample = (%d, %q, %q), want exit 0 and no debt", code, stdout, stderr)
	}

	appendFile(t, filepath.Join(dir, ".workflow/specs/orders/SPEC.md"), "\nSee ADR-0002.\n")
	if code, stdout, stderr := runWith(t, app(), "spec-coverage"); code != 1 || !strings.Contains(stdout, "ADR-0002 is mentioned without a link") || !strings.Contains(stderr, "1 problem") {
		t.Errorf("wf spec-coverage with a problem = (%d, %q, %q), want exit 1 naming it", code, stdout, stderr)
	}
	if code, stdout, _ := runWith(t, app(), "spec-coverage", "--report-only"); code != 0 || !strings.Contains(stdout, "ADR-0002 is mentioned without a link") || !strings.Contains(stdout, "Report-only") {
		t.Errorf("wf spec-coverage --report-only = (%d, %q), want exit 0 and the same problem reported", code, stdout)
	}
}

func appendFile(t *testing.T, name, text string) {
	t.Helper()
	file, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

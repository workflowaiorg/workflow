package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/workflowaiorg/workflow/internal/cli"
	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/project"
)

// sample is a component as a real one would be, with a part of every kind.
var sample = component.Component{
	Name:    "sample",
	Summary: "A component for tests",
	Options: []component.Option{
		{Name: "db", Question: "Which database", Default: "postgres", Choices: []string{"postgres", "sqlite"}},
		{Name: "grpc", Question: "Serve gRPC too", Bool: true, Default: "false"},
	},
	Base: fstest.MapFS{
		"AGENTS.md": {Data: []byte("## Sample\n\nThe sample component's rules.\n")},
		"skills/sample/SKILL.md": {Data: []byte("---\nname: sample\ndescription: \"Using the sample component\"\n---\n\n" +
			"Database: [[ .components.sample.db ]].\n[[ range commands \"sample\" ]]\n- `[[ .Usage ]]`: [[ .Summary ]][[ end ]]\n")},
	},
	Skeleton: fstest.MapFS{"sample/config.txt": {Data: []byte("the project's own\n")}},
	Gate:     []project.GateStep{{Name: "sample test", Paths: []string{"sample/**"}, Run: "wf sample hello"}},
	Doctor:   []project.Check{{Name: "sample tool installed", Run: "true"}},
	Commands: []component.Command{{
		Name: "hello", Usage: "wf sample hello", Summary: "Say hello from the database",
		Run: func(ctx component.Context, args []string) error {
			_, err := fmt.Fprintf(ctx.Stdout, "hello from %s\n", ctx.Answers["db"])
			return err
		},
	}},
	Edits: func(answers map[string]any) []string {
		return []string{"add /sample/cache to .gitignore"}
	},
}

// runSample runs wf on the project at dir, offering the sample component.
func runSample(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	app := cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
	app.SetComponents(sample)
	return runWith(t, app, args...)
}

func initialised(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if code, _, stderr := runSample(t, dir, "init", "--forge=gitlab"); code != 0 {
		t.Fatalf("wf init exited %d: %s", code, stderr)
	}
	return dir
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestAddInstallsEveryPartOfAComponent(t *testing.T) {
	dir := initialised(t)
	code, stdout, stderr := runSample(t, dir, "add", "sample", "--db=sqlite", "--grpc")
	if code != 0 {
		t.Fatalf("wf add sample = (%d, %q)", code, stderr)
	}

	config := read(t, dir, ".wf.yml")
	if !strings.Contains(config, "components:\n  sample:\n    db: sqlite\n    grpc: true\n") {
		t.Errorf(".wf.yml does not record the answers:\n%s", config)
	}
	if !strings.Contains(config, "# gate:") {
		t.Errorf(".wf.yml lost its comments:\n%s", config)
	}
	skill := read(t, dir, ".claude/skills/sample/SKILL.md")
	for _, want := range []string{"Database: sqlite.", "- `wf sample hello`: Say hello from the database"} {
		if !strings.Contains(skill, want) {
			t.Errorf("the sample skill does not say %q:\n%s", want, skill)
		}
	}
	if agents := read(t, dir, "AGENTS.md"); !strings.Contains(agents, "## Sample\n\nThe sample component's rules.") {
		t.Errorf("AGENTS.md has no section of the component's:\n%s", agents)
	}
	// The component's steps come after wf's own and before the project's.
	appendFile(t, filepath.Join(dir, ".wf.yml"), "gate:\n  - name: lint\n    paths: [\"**/*.go\"]\n    run: make lint\n")
	if code, _, stderr := runSample(t, dir, "sync"); code != 0 {
		t.Fatalf("wf sync = (%d, %q)", code, stderr)
	}
	check := read(t, dir, ".claude/commands/pre-mr-check.md")
	specs := strings.Index(check, "| specs and records |")
	step := strings.Index(check, "| sample test | `sample/**` | `wf sample hello` |")
	lint := strings.Index(check, "| lint |")
	if specs < 0 || step < specs || lint < step {
		t.Errorf("the gate table is not wf's steps, the component's, then the project's:\n%s", check)
	}
	if got := read(t, dir, "sample/config.txt"); got != "the project's own\n" {
		t.Errorf("sample/config.txt = %q, want the component's skeleton", got)
	}
	if !strings.Contains(stdout, "wrote    sample/config.txt") || !strings.Contains(stderr, "by hand  add /sample/cache to .gitignore") {
		t.Errorf("wf add printed (%q, %q), want the skeleton written and the edit left to the developer", stdout, stderr)
	}

	if code, stdout, stderr := runSample(t, dir, "sample", "hello"); code != 0 || stdout != "hello from sqlite\n" {
		t.Errorf("wf sample hello = (%d, %q, %q), want the command run with its answers", code, stdout, stderr)
	}
	if _, stdout, _ := runSample(t, dir, "help"); !strings.Contains(stdout, "wf sample hello") {
		t.Errorf("wf help does not list the component's command:\n%s", stdout)
	}
	if code, _, stderr := runSample(t, dir, "sync", "--check"); code != 0 {
		t.Errorf("wf sync --check after wf add = (%d, %q), want it up to date", code, stderr)
	}
	if code, _, stderr := runSample(t, dir, "add", "sample", "--db=sqlite", "--grpc=false"); code != 2 || !strings.Contains(stderr, "installed already") {
		t.Errorf("a second wf add sample = (%d, %q), want exit 2 saying it is installed", code, stderr)
	}
}

func TestAddAsksWhatNoFlagSays(t *testing.T) {
	dir := initialised(t)
	if code, _, stderr := runSample(t, dir, "add", "sample", "--grpc"); code != 2 || !strings.Contains(stderr, "pass --db") {
		t.Errorf("wf add with no terminal = (%d, %q), want exit 2 naming --db", code, stderr)
	}
	if strings.Contains(read(t, dir, ".wf.yml"), "components:") {
		t.Error("a wf add that asked nothing recorded the component")
	}
	if code, _, stderr := runSample(t, dir, "add", "sample", "--db=mysql", "--grpc"); code != 2 || !strings.Contains(stderr, "one of postgres, sqlite") {
		t.Errorf("wf add --db=mysql = (%d, %q), want exit 2 naming the choices", code, stderr)
	}

	app := cli.New("0.1.0", dir, strings.NewReader("\nyes\n"), new(bytes.Buffer), new(bytes.Buffer))
	app.SetComponents(sample)
	app.SetInteractive()
	if code, _, stderr := runWith(t, app, "add", "sample"); code != 0 {
		t.Fatalf("wf add answered on a terminal = (%d, %q)", code, stderr)
	}
	if config := read(t, dir, ".wf.yml"); !strings.Contains(config, "db: postgres\n    grpc: true") {
		t.Errorf("an empty answer did not take the default, or yes was not true:\n%s", config)
	}
}

func TestAddRefusesAComponentWhosePartIsTaken(t *testing.T) {
	dir := initialised(t)
	clash := sample
	clash.Name = "clash"
	clash.Options = nil
	clash.Base = fstest.MapFS{"skills/workflow/SKILL.md": {Data: []byte("---\nname: workflow\ndescription: \"x\"\n---\n")}}
	app := cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
	app.SetComponents(clash)
	before := read(t, dir, ".agents/base/skills/workflow/SKILL.md")
	if code, _, stderr := runWith(t, app, "add", "clash"); code != 1 || !strings.Contains(stderr, "skills/workflow/SKILL.md") {
		t.Errorf("wf add clash = (%d, %q), want a failure naming the file", code, stderr)
	}
	if after := read(t, dir, ".agents/base/skills/workflow/SKILL.md"); after != before || strings.Contains(read(t, dir, ".wf.yml"), "clash") {
		t.Error("a wf add that failed changed the base or .wf.yml")
	}
}

func TestAComponentWithNoCommandsSaysSo(t *testing.T) {
	dir := initialised(t)
	quiet := sample
	quiet.Name = "quiet"
	quiet.Options = nil
	quiet.Base = nil
	quiet.Skeleton = nil
	quiet.Commands = nil
	app := cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
	app.SetComponents(quiet)
	if code, _, stderr := runWith(t, app, "add", "quiet"); code != 0 {
		t.Fatalf("wf add quiet exited %d: %s", code, stderr)
	}
	for _, args := range [][]string{{"quiet"}, {"quiet", "-h"}} {
		if code, _, stderr := runWith(t, app, args...); code != 2 || !strings.Contains(stderr, "has no commands") {
			t.Errorf("wf %s = (%d, %q), want exit 2 saying it has no commands", strings.Join(args, " "), code, stderr)
		}
	}
}

func TestAddRefusesComponentsItCannotAddTo(t *testing.T) {
	dir := initialised(t)
	config := read(t, dir, ".wf.yml") + "saved: &installed {}\ncomponents: *installed\n"
	if err := os.WriteFile(filepath.Join(dir, ".wf.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	before := read(t, dir, "AGENTS.md")
	if code, _, stderr := runSample(t, dir, "add", "sample", "--db=sqlite", "--grpc"); code != 2 || !strings.Contains(stderr, "written out in full") {
		t.Errorf("wf add over an aliased components: = (%d, %q), want exit 2 asking for it in full", code, stderr)
	}
	if read(t, dir, ".wf.yml") != config || read(t, dir, "AGENTS.md") != before {
		t.Error("a refused wf add changed .wf.yml or the files wf writes")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents/base/skills/sample")); err == nil {
		t.Error("a refused wf add installed the component's part of the base")
	}
}

func TestComponentNamesAreChecked(t *testing.T) {
	dir := initialised(t)
	if code, _, stderr := runSample(t, dir, "sample", "hello"); code != 2 || !strings.Contains(stderr, "wf add sample installs it") {
		t.Errorf("a command of a component not installed = (%d, %q), want exit 2 saying how to install it", code, stderr)
	}
	if code, _, stderr := runSample(t, dir, "add", "nothing"); code != 2 || !strings.Contains(stderr, "it knows sample") {
		t.Errorf("wf add nothing = (%d, %q), want exit 2 naming what wf knows", code, stderr)
	}
	if code, stdout, _ := runSample(t, dir, "add"); code != 0 || !strings.Contains(stdout, "sample") {
		t.Errorf("wf add alone = (%d, %q), want the components listed", code, stdout)
	}
	appendFile(t, filepath.Join(dir, ".wf.yml"), "components:\n  nothing: {}\n")
	if code, _, stderr := runSample(t, dir, "sync"); code != 2 || !strings.Contains(stderr, "components.nothing is not a component wf knows") {
		t.Errorf("wf sync with an unknown component = (%d, %q), want exit 2 naming it", code, stderr)
	}
}

func TestAddRefusesAnotherVersionsBase(t *testing.T) {
	dir := initialised(t)
	config := filepath.Join(dir, ".wf.yml")
	if err := os.WriteFile(config, []byte(strings.Replace(read(t, dir, ".wf.yml"), "wf: 0.1.0", "wf: 0.0.9", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	before := read(t, dir, ".agents/base/AGENTS.md")
	if code, _, stderr := runSample(t, dir, "add", "sample", "--db=sqlite", "--grpc"); code != 2 || !strings.Contains(stderr, "pins wf 0.0.9, but this is wf 0.1.0") {
		t.Errorf("wf add under another pin = (%d, %q), want exit 2 naming both versions", code, stderr)
	}
	if read(t, dir, ".agents/base/AGENTS.md") != before || strings.Contains(read(t, dir, ".wf.yml"), "components:") {
		t.Error("wf add under another pin changed the base or .wf.yml")
	}
}

func TestAnswersEditedByHandAreChecked(t *testing.T) {
	tests := []struct{ answers, want string }{
		{"db: mysql\n    grpc: true", "components.sample.db is one of postgres, sqlite"},
		{"db: sqlite\n    grpc: maybe", "components.sample.grpc is true or false"},
		{"db: sqlite", "components.sample.grpc has no answer"},
		{"db: sqlite\n    grpc: true\n    grcp: true", "components.sample.grcp is not an option"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			dir := initialised(t)
			appendFile(t, filepath.Join(dir, ".wf.yml"), "components:\n  sample:\n    "+test.answers+"\n")
			if code, _, stderr := runSample(t, dir, "sync"); code != 2 || !strings.Contains(stderr, test.want) {
				t.Errorf("wf sync = (%d, %q), want exit 2 saying %q", code, stderr, test.want)
			}
		})
	}
}

func TestAddListsAComponentsOptions(t *testing.T) {
	code, stdout, _ := runSample(t, t.TempDir(), "add", "sample", "-h")
	for _, want := range []string{"usage: wf add sample", "--db=<answer>", "postgres, sqlite; default postgres", "--grpc", "yes or no; default false"} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Errorf("wf add sample -h = (%d, %q), want it to say %q", code, stdout, want)
		}
	}
}

func TestNoComponentIsNamedLikeACommand(t *testing.T) {
	// wf runs its own command first, so a component named like one would lose its commands.
	_, stdout, _ := run(t, "help")
	for _, c := range component.All {
		if strings.Contains(stdout, "wf "+c.Name+" ") {
			t.Errorf("the %s component is named like a wf command", c.Name)
		}
	}
}

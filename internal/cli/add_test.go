package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
		{
			Name: "grpc", Question: "Serve gRPC too", Bool: true, Default: "false",
			Part: component.Part{
				Base:     fstest.MapFS{"skills/sample-grpc/SKILL.md": {Data: []byte("---\nname: sample-grpc\ndescription: \"Serving gRPC\"\n---\n\nProtos.\n")}},
				Skeleton: fstest.MapFS{"sample/protos.txt": {Data: []byte("protos\n")}},
				Gate:     []project.GateStep{{Name: "sample protos", Paths: []string{"sample/**"}, Run: "sample-protos"}},
				Doctor:   []project.Check{{Name: "sample protoc installed", Run: "true"}},
			},
		},
	},
	Part: component.Part{
		Base: fstest.MapFS{
			"AGENTS.md": {Data: []byte("## Sample\n\nThe sample component's rules.\n")},
			"skills/sample/SKILL.md": {Data: []byte("---\nname: sample\ndescription: \"Using the sample component\"\n---\n\n" +
				"Database: [[ .components.sample.db ]].\n")},
		},
		Skeleton: fstest.MapFS{"sample/config.txt": {Data: []byte("the project's own\n")}},
		Gate:     []project.GateStep{{Name: "sample test", Paths: []string{"sample/**"}, Run: "sample-test"}},
		Doctor:   []project.Check{{Name: "sample tool installed", Run: "true"}},
	},
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
	if !strings.Contains(skill, "Database: sqlite.") {
		t.Errorf("the sample skill does not hold its answer:\n%s", skill)
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
	step := strings.Index(check, "| sample test | `sample/**` | `sample-test` |")
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

	if code, _, stderr := runSample(t, dir, "sync", "--check"); code != 0 {
		t.Errorf("wf sync --check after wf add = (%d, %q), want it up to date", code, stderr)
	}
	optionParts(t, dir, true)
	if code, _, stderr := runSample(t, dir, "add", "sample", "--db=sqlite", "--grpc=false"); code != 2 || !strings.Contains(stderr, "installed already") {
		t.Errorf("a second wf add sample = (%d, %q), want exit 2 saying it is installed", code, stderr)
	}
}

func TestAnOptionAnsweredNoBringsNothing(t *testing.T) {
	dir := initialised(t)
	if code, _, stderr := runSample(t, dir, "add", "sample", "--db=sqlite", "--grpc=false"); code != 0 {
		t.Fatalf("wf add sample = (%d, %q)", code, stderr)
	}
	optionParts(t, dir, false)
	if code, _, stderr := runSample(t, dir, "sync", "--check"); code != 0 {
		t.Errorf("wf sync --check after wf add = (%d, %q), want it up to date", code, stderr)
	}
}

// optionParts checks that every part of the sample's grpc option is in the project at dir when
// chosen, and that none of it is when not.
func optionParts(t *testing.T, dir string, chosen bool) {
	t.Helper()
	for _, name := range []string{".agents/base/skills/sample-grpc/SKILL.md", ".claude/skills/sample-grpc/SKILL.md", "sample/protos.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err == nil) != chosen {
			t.Errorf("with grpc %v, %s exists = %v", chosen, name, err == nil)
		}
	}
	if got := strings.Contains(read(t, dir, ".claude/commands/pre-mr-check.md"), "| sample protos |"); got != chosen {
		t.Errorf("with grpc %v, the gate has the option's step = %v", chosen, got)
	}
	_, stdout, _ := runSample(t, dir, "doctor")
	if got := strings.Contains(stdout, "sample protoc installed"); got != chosen {
		t.Errorf("with grpc %v, wf doctor runs the option's check = %v:\n%s", chosen, got, stdout)
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

func TestAddWritesNothingWhenATemplateFails(t *testing.T) {
	dir := initialised(t)
	broken := sample
	broken.Name = "broken"
	broken.Options = nil
	broken.Skeleton = fstest.MapFS{
		"broken/a.txt":      {Data: []byte("written first\n")},
		"broken/b.txt.tmpl": {Data: []byte("[[ .missing ]]\n")},
	}
	app := cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
	app.SetComponents(broken)
	config := read(t, dir, ".wf.yml")
	if code, _, stderr := runWith(t, app, "add", "broken"); code != 1 || !strings.Contains(stderr, "b.txt.tmpl") {
		t.Errorf("wf add broken = (%d, %q), want a failure naming the template", code, stderr)
	}
	if read(t, dir, ".wf.yml") != config {
		t.Error("a wf add that failed changed .wf.yml")
	}
	for _, path := range []string{".agents/base/skills/sample", "broken/a.txt"} {
		if _, err := os.Stat(filepath.Join(dir, path)); err == nil {
			t.Errorf("a wf add whose template failed wrote %s", path)
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
	if code, _, stderr := runSample(t, dir, "add", "nothing"); code != 2 || !strings.Contains(stderr, "it knows sample") {
		t.Errorf("wf add nothing = (%d, %q), want exit 2 naming what wf knows", code, stderr)
	}
	if code, stdout, _ := runSample(t, dir, "add"); code != 0 || !strings.Contains(stdout, "sample") {
		t.Errorf("wf add alone = (%d, %q), want the components listed", code, stdout)
	}
	if code, stdout, _ := runSample(t, t.TempDir(), "add"); code != 0 || !strings.Contains(stdout, "sample") {
		t.Errorf("wf add alone before wf init = (%d, %q), want the components listed", code, stdout)
	}
	appendFile(t, filepath.Join(dir, ".wf.yml"), "components:\n  nothing: {}\n")
	if code, stdout, stderr := runSample(t, dir, "add"); code != 2 || !strings.Contains(stderr, "components.nothing") {
		t.Errorf("wf add alone with a broken %s = (%d, %q, %q), want exit 2 naming the problem", project.ConfigFile, code, stdout, stderr)
	}
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

// TestAProjectWithTheBackendBuildsAndPasses adds the backend to a new project, with no option and
// with every one, then downloads the modules its code imports and runs go vet and go test there, as
// a developer would. It needs the network, so -short skips it.
func TestAProjectWithTheBackendBuildsAndPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads modules")
	}
	for _, every := range []bool{false, true} {
		t.Run(fmt.Sprintf("every option %v", every), func(t *testing.T) {
			dir := t.TempDir()
			app := cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
			if code, _, stderr := runWith(t, app, "init", "--forge=gitlab"); code != 0 {
				t.Fatalf("wf init exited %d: %s", code, stderr)
			}
			app = cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
			code, _, stderr := runWith(t, app, "add", "backend", "--module=example.com/shop", fmt.Sprintf("--postgres=%v", every), fmt.Sprintf("--queue=%v", every), fmt.Sprintf("--llm-agents=%v", every))
			if code != 0 {
				t.Fatalf("wf add backend exited %d: %s", code, stderr)
			}
			if !strings.Contains(stderr, "by hand  run go mod tidy") {
				t.Errorf("wf add backend does not say to run go mod tidy:\n%s", stderr)
			}
			for _, skill := range []string{"architecture", "api-design", "configuration", "error-handling", "shared-packages", "testing", "go-style"} {
				if _, err := os.Stat(filepath.Join(dir, ".claude/skills", skill, "SKILL.md")); err != nil {
					t.Errorf("the %s skill was not installed: %v", skill, err)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, ".claude/commands/backend-design.md")); err != nil {
				t.Errorf("the backend-design command was not installed: %v", err)
			}
			for _, skill := range []string{"database", "queue", "llm-agents"} {
				if _, err := os.Stat(filepath.Join(dir, ".claude/skills", skill, "SKILL.md")); (err == nil) != every {
					t.Errorf("with every option %v, the %s skill is installed = %v", every, skill, err == nil)
				}
			}

			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "")
			// The project's own Go version, as CI has it: a module that needs a newer Go fails here
			// rather than upgrading the toolchain unseen.
			t.Setenv("GOTOOLCHAIN", "local")
			// vet with the integration tag too, so the tests that need Docker at least build.
			for _, args := range [][]string{{"mod", "tidy"}, {"vet", "./..."}, {"vet", "-tags=integration", "./..."}, {"test", "./..."}} {
				command := exec.Command("go", args...)
				command.Dir = dir
				if out, err := command.CombinedOutput(); err != nil {
					t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
			for _, module := range []string{"github.com/uptrace/bun", "github.com/testcontainers/testcontainers-go", "github.com/redis/go-redis/v9", "github.com/a2aproject/a2a-go/v2", "github.com/openai/openai-go/v3"} {
				if got := strings.Contains(read(t, dir, "go.mod"), module); got != every {
					t.Errorf("with every option %v, go.mod requires %s = %v", every, module, got)
				}
			}
		})
	}
}

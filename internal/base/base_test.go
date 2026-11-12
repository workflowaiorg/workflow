package base_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/component/backend"
	"github.com/workflowaiorg/workflow/internal/project"
)

func TestFilesRefusesAForgeItHasNoBaseFor(t *testing.T) {
	if _, err := base.Files("sourcehut", nil); err == nil || !strings.Contains(err.Error(), "sourcehut") {
		t.Errorf("Files(sourcehut) error = %v, want one naming the forge", err)
	}
}

func TestOnlyTheForgeSkillNamesTheForge(t *testing.T) {
	// Everything forge-specific lives in the forge's own skill, so the rest holds on any forge.
	forgeWords := regexp.MustCompile(`\bglab\b|\bgh\b|\w::\w|(?i:merge requests?|pull requests?)\b|\bPRs?\b|\bGit(Lab|Hub)\b`)
	cli := map[string]string{"gitlab": "glab", "github": "gh"}
	// The backend with every option on, so that none of its parts goes unread.
	all := map[string]any{}
	for _, option := range backend.Component.Options {
		if option.Bool {
			all[option.Name] = true
		}
	}
	for _, forge := range project.Forges {
		files, err := base.Files(forge, []component.Component{backend.Component.With(all)})
		if err != nil {
			t.Fatal(err)
		}
		skill := "skills/forge-" + forge + "/SKILL.md"
		if !regexp.MustCompile(`\b` + cli[forge] + ` `).Match(files[skill]) {
			t.Errorf("the forge-%s skill never runs %s", forge, cli[forge])
		}
		for name, content := range files {
			if name == skill {
				continue
			}
			if found := forgeWords.FindString(string(content)); found != "" {
				t.Errorf("on %s, %s names %q, which belongs to the forge skill", forge, name, found)
			}
		}
	}
}

// installed returns a directory holding a project's .wf.yml and the base of its forge.
func installed(t *testing.T, pin string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".wf.yml"), []byte("wf: "+pin+"\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := base.Install(dir, "gitlab", nil); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckNamesEveryFileThatIsNotTheBase(t *testing.T) {
	dir := installed(t, "1.2.0")
	if differences, err := check(t, dir, "1.2.0"); err != nil || len(differences) != 0 {
		t.Fatalf("Check right after Install = (%v, %v), want no differences", differences, err)
	}

	edit := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, ".agents/base", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	edit("skills/workflow/SKILL.md", "edited by hand\n")
	edit("skills/ours/SKILL.md", "a project's own skill, in the wrong place\n")
	if err := os.Remove(filepath.Join(dir, ".agents/base/commands/plan-check.md")); err != nil {
		t.Fatal(err)
	}
	// Windows line endings are not a difference.
	content, err := os.ReadFile(filepath.Join(dir, ".agents/base/workflow/GUIDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	edit("workflow/GUIDE.md", strings.ReplaceAll(string(content), "\n", "\r\n"))

	differences, err := check(t, dir, "1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".agents/base/commands/plan-check.md: missing",
		".agents/base/skills/workflow/SKILL.md: differs",
		".agents/base/skills/ours/SKILL.md: not part of",
	}
	if len(differences) != len(want) {
		t.Fatalf("Check = %q, want %d differences", differences, len(want))
	}
	for _, prefix := range want {
		if !slices.ContainsFunc(differences, func(line string) bool { return strings.HasPrefix(line, prefix) }) {
			t.Errorf("Check = %q, want a line starting %q", differences, prefix)
		}
	}
}

func TestCheckNeedsThePinnedVersion(t *testing.T) {
	dir := installed(t, "1.2.0")
	differences, err := check(t, dir, "1.3.0")
	if err != nil || len(differences) != 1 || !strings.Contains(differences[0], "pins wf 1.2.0, but this is wf 1.3.0") {
		t.Errorf("Check by another version = (%q, %v), want one line naming both versions", differences, err)
	}
}

func TestSkeletonLeavesWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, ".workflow/decisions/README.md")
	written, err := base.Skeleton(dir)
	if err != nil || !slices.Contains(written, ".workflow/decisions/README.md") {
		t.Fatalf("Skeleton = (%v, %v), want the decisions index written", written, err)
	}
	if err := os.WriteFile(index, []byte("the project's own index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if written, err := base.Skeleton(dir); err != nil || len(written) != 0 {
		t.Errorf("Skeleton over an existing index = (%v, %v), want nothing written", written, err)
	}
	if content, _ := os.ReadFile(index); string(content) != "the project's own index\n" {
		t.Errorf("the project's index was overwritten: %q", content)
	}
}

// check compares the base of the project at dir, read with its own settings, as version would.
func check(t *testing.T, dir, version string) ([]string, error) {
	t.Helper()
	config, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return base.Check(dir, config, nil, version)
}

func TestFilesAddsEachComponentsPart(t *testing.T) {
	core, err := base.Files("gitlab", nil)
	if err != nil {
		t.Fatal(err)
	}
	part := func(name string, files map[string]string) component.Component {
		fsys := fstest.MapFS{}
		for path, content := range files {
			fsys[path] = &fstest.MapFile{Data: []byte(content)}
		}
		return component.Component{Name: name, Part: component.Part{Base: fsys}}
	}
	a := part("a", map[string]string{"AGENTS.md": "## A\n", "workflow/GUIDE.md": "## A's guide\n", "skills/a/SKILL.md": "a"})
	b := part("b", map[string]string{"AGENTS.md": "## B\n"})

	files, err := base.Files("gitlab", []component.Component{a, b})
	if err != nil {
		t.Fatal(err)
	}
	trimmed := func(name string) string { return strings.TrimRight(string(core[name]), "\n") }
	if got, want := string(files["AGENTS.md"]), trimmed("AGENTS.md")+"\n\n## A\n\n## B\n"; got != want {
		t.Errorf("AGENTS.md ends %q, want wf's, then A's section, then B's", got[max(0, len(got)-40):])
	}
	if got, want := string(files["workflow/GUIDE.md"]), trimmed("workflow/GUIDE.md")+"\n\n## A's guide\n"; got != want {
		t.Errorf("workflow/GUIDE.md does not end with A's section")
	}
	if string(files["skills/a/SKILL.md"]) != "a" {
		t.Error("A's own skill is not in the base")
	}

	clash := part("clash", map[string]string{"skills/a/SKILL.md": "mine"})
	if _, err := base.Files("gitlab", []component.Component{a, clash}); err == nil || !strings.Contains(err.Error(), "skills/a/SKILL.md") {
		t.Errorf("Files with two components owning one file = %v, want an error naming it", err)
	}
}

func TestRenderComponentSkeletonRendersItsTemplates(t *testing.T) {
	skeleton := component.Component{Name: "sample", Part: component.Part{Skeleton: fstest.MapFS{
		"go.mod.tmpl":  {Data: []byte("module [[ .module ]]\n")},
		"main.go.tmpl": {Data: []byte("package main\n\nimport _ \"[[ .module ]]/pkg\"\n")},
		"README.md":    {Data: []byte("Written as it is: [[ not a template ]]\n")},
		"kept.txt":     {Data: []byte("wf's\n")},
		"kept.go.tmpl": {Data: []byte("package kept // wf's\n")},
	}}}
	dir := t.TempDir()
	for name, content := range map[string]string{"kept.txt": "the project's\n", "kept.go": "package kept // the project's\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rendered, err := base.RenderComponentSkeleton(skeleton, map[string]any{"module": "example.com/shop"})
	if err != nil {
		t.Fatal(err)
	}
	written, existing, err := base.WriteSkeleton(dir, rendered)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md", "go.mod", "main.go"}; !slices.Equal(written, want) {
		t.Errorf("wrote %v, want %v", written, want)
	}
	if want := []string{"kept.go", "kept.txt"}; !slices.Equal(existing, want) {
		t.Errorf("left alone %v, want %v", existing, want)
	}
	for name, want := range map[string]string{
		"go.mod":    "module example.com/shop\n",
		"main.go":   "package main\n\nimport _ \"example.com/shop/pkg\"\n",
		"README.md": "Written as it is: [[ not a template ]]\n",
		"kept.txt":  "the project's\n",
		"kept.go":   "package kept // the project's\n",
	} {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(content) != want {
			t.Errorf("%s = %q, %v; want %q", name, content, err, want)
		}
	}
}

func TestRenderComponentSkeletonRefusesABrokenTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     string
	}{
		{name: "an answer it was not given", template: "[[ .missing ]]", want: `map has no entry for key "missing"`},
		{name: "a template that does not parse", template: "[[ .module ", want: "reading the sample component's broken.tmpl"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			skeleton := component.Component{Name: "sample", Part: component.Part{Skeleton: fstest.MapFS{"broken.tmpl": {Data: []byte(test.template)}}}}
			_, err := base.RenderComponentSkeleton(skeleton, map[string]any{"module": "example.com/shop"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("RenderComponentSkeleton = %v, want it to say %q", err, test.want)
			}
		})
	}
}

func TestRenderComponentSkeletonRefusesTwoPartsWithTheSameFile(t *testing.T) {
	sample := component.Component{
		Name: "sample",
		Part: component.Part{Skeleton: fstest.MapFS{"x.txt": {Data: []byte("the component's\n")}}},
		Options: []component.Option{{Name: "extra", Bool: true, Default: "false", Part: component.Part{
			Skeleton: fstest.MapFS{"x.txt": {Data: []byte("the option's\n")}},
		}}},
	}
	for _, test := range []struct {
		extra bool
		want  string
	}{
		{extra: false},
		{extra: true, want: "two of its parts both have x.txt"},
	} {
		_, err := base.RenderComponentSkeleton(sample, map[string]any{"extra": test.extra})
		switch {
		case test.want == "" && err != nil:
			t.Errorf("extra=%v: RenderComponentSkeleton = %v, want no error", test.extra, err)
		case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
			t.Errorf("extra=%v: RenderComponentSkeleton = %v, want it to say %q", test.extra, err, test.want)
		}
	}
}

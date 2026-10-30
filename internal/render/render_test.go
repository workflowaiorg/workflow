package render_test

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/project"
	"github.com/workflowaiorg/workflow/internal/render"
)

var update = flag.Bool("update", false, "rewrite testdata/sync/want from what wf sync writes now")

const version = "0.1.0"

// sample copies the sample project into a new directory and returns it.
func sample(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/sync/project")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// tree returns every file under root by its slash-separated path, skipping the paths in skip.
func tree(t *testing.T, root string, skip ...string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(full string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, full)
		rel = filepath.ToSlash(rel)
		if slices.ContainsFunc(skip, func(prefix string) bool { return strings.HasPrefix(rel, prefix) }) {
			return nil
		}
		content, err := os.ReadFile(full)
		files[rel] = content
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestSyncGolden runs wf sync on the sample project and compares every file it writes with
// testdata/sync/want. After an intended change, go test ./internal/render -update rewrites want.
func TestSyncGolden(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	got := tree(t, dir, ".agents/", project.ConfigFile)

	const wantDir = "testdata/sync/want"
	if *update {
		if err := os.RemoveAll(wantDir); err != nil {
			t.Fatal(err)
		}
		if err := os.CopyFS(wantDir, os.DirFS(dir)); err != nil {
			t.Fatal(err)
		}
		for _, input := range []string{".agents", project.ConfigFile} {
			if err := os.RemoveAll(filepath.Join(wantDir, input)); err != nil {
				t.Fatal(err)
			}
		}
	}
	want := tree(t, wantDir)

	for path, content := range want {
		if !bytes.Equal(got[path], content) {
			t.Errorf("%s:\n--- got ---\n%s\n--- want ---\n%s", path, got[path], content)
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("%s was written but is not in %s", path, wantDir)
		}
	}
}

func TestCheckNamesAHandEditedFile(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	if differences, err := checkProject(dir); err != nil || len(differences) > 0 {
		t.Fatalf("Check right after Sync = (%v, %v), want nothing", differences, err)
	}

	edited := filepath.Join(dir, ".github/skills/hello/SKILL.md")
	if err := os.WriteFile(edited, []byte("edited by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	differences, err := checkProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(differences) != 1 || !strings.HasPrefix(differences[0], ".github/skills/hello/SKILL.md: ") {
		t.Errorf("Check = %q, want one line naming .github/skills/hello/SKILL.md", differences)
	}
}

func TestSyncLeavesTheProjectPartAlone(t *testing.T) {
	dir := sample(t)
	before := tree(t, filepath.Join(dir, project.ProjectDir))
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	after := tree(t, filepath.Join(dir, project.ProjectDir))

	if len(after) != len(before) {
		t.Errorf("project part had %d files before sync and %d after", len(before), len(after))
	}
	for path, content := range before {
		if !bytes.Equal(after[path], content) {
			t.Errorf("sync changed %s/%s", project.ProjectDir, path)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, project.ProjectDir, "skills/notes/SKILL.md")); err == nil {
		t.Error("sync created a project part for notes, which has none")
	}
}

func TestSyncRemovesOnlyWhatItGenerated(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	ownSkill := filepath.Join(dir, ".claude/skills/handmade/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(ownSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownSkill, []byte("a skill wf never wrote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, project.BaseDir, "skills/notes")); err != nil {
		t.Fatal(err)
	}

	report, err := syncProject(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wantRemoved := []string{".claude/skills/notes/SKILL.md", ".github/skills/notes/SKILL.md", ".opencode/skills/notes/SKILL.md"}
	if !slices.Equal(report.Removed, wantRemoved) {
		t.Errorf("removed %v, want %v", report.Removed, wantRemoved)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude/skills/notes")); err == nil {
		t.Error("the emptied .claude/skills/notes directory is still there")
	}
	if _, err := os.Stat(ownSkill); err != nil {
		t.Errorf("sync removed a file it never generated: %v", err)
	}
}

func TestSyncRefusesALockListingAFileItNeverGenerates(t *testing.T) {
	for _, path := range []string{
		"../outside.md",
		"./.agents/project/AGENTS.md",
		".agents/base/../project/skills/hello/SKILL.md",
		".claude/skills/../../.wf.yml",
		".wf.yml",
		"go.mod",
		".github/workflows/ci.yml",
		".claude/agents/notes.txt",
		`.claude/skills/..\..\x.md`,
	} {
		t.Run(path, func(t *testing.T) {
			dir := sample(t)
			lock := "wf: 0.1.0\nfiles:\n  " + path + ": sha256:00\n"
			if err := os.WriteFile(filepath.Join(dir, project.LockFile), []byte(lock), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := syncProject(dir, false); err == nil || !strings.Contains(err.Error(), "is not a file wf generates") {
				t.Errorf("Sync = %v, want the lock entry refused", err)
			}
			if _, err := os.Stat(filepath.Join(dir, project.ConfigFile)); err != nil {
				t.Errorf("Sync removed %s: %v", project.ConfigFile, err)
			}
		})
	}
}

func TestSyncSkipsWhatWasNotItsOwnUnlessForced(t *testing.T) {
	dir := sample(t)
	// AGENTS.md was the project's before wf.
	own := []byte("the project's own\n")
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), own, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := syncProject(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.Skipped, []string{"AGENTS.md"}) {
		t.Errorf("sync skipped %v, want [AGENTS.md]", report.Skipped)
	}
	if content, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md")); !bytes.Equal(content, own) {
		t.Errorf("sync without --force overwrote AGENTS.md:\n%s", content)
	}
	differences, err := checkProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(differences, func(line string) bool { return strings.HasPrefix(line, "AGENTS.md: not written by wf") }) {
		t.Errorf("Check = %q, want AGENTS.md named as not written by wf", differences)
	}

	report, err = syncProject(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.Replaced, []string{"AGENTS.md"}) {
		t.Errorf("sync --force replaced %v, want [AGENTS.md]", report.Replaced)
	}
	if differences, err := checkProject(dir); err != nil || len(differences) > 0 {
		t.Errorf("Check after sync --force = (%q, %v), want nothing", differences, err)
	}
}

func TestSyncRefusesToWriteThroughASymlink(t *testing.T) {
	links := map[string]string{
		// A whole assistant folder linked to the project's part.
		".claude/skills": "../.agents/project/skills",
		// A single generated file linked to the project's part.
		"AGENTS.md": ".agents/project/AGENTS.md",
	}
	for link, target := range links {
		t.Run(link, func(t *testing.T) {
			dir := sample(t)
			before := tree(t, filepath.Join(dir, project.ProjectDir))
			full := filepath.Join(dir, filepath.FromSlash(link))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, full); err != nil {
				t.Fatal(err)
			}

			_, err := syncProject(dir, true)
			if err == nil || !strings.Contains(err.Error(), link+" is a symbolic link") {
				t.Errorf("Sync = %v, want an error naming %s", err, link)
			}
			after := tree(t, filepath.Join(dir, project.ProjectDir))
			if len(after) != len(before) {
				t.Errorf("project part had %d files before and %d after", len(before), len(after))
			}
			for path, content := range before {
				if !bytes.Equal(after[path], content) {
					t.Errorf("sync wrote %s/%s through the link", project.ProjectDir, path)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, project.LockFile)); err == nil {
				t.Error("sync wrote .wf.lock although it refused")
			}
		})
	}
}

func TestSyncReplacesItsOwnFileEditedByHand(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(dir, ".claude/skills/hello/SKILL.md")
	if err := os.WriteFile(edited, []byte("edited by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := syncProject(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.Replaced, []string{".claude/skills/hello/SKILL.md"}) {
		t.Errorf("replaced %v, want the hand-edited hello skill", report.Replaced)
	}
	got, _ := os.ReadFile(edited)
	want, _ := os.ReadFile("testdata/sync/want/.claude/skills/hello/SKILL.md")
	if !bytes.Equal(got, want) {
		t.Errorf("hello skill after sync:\n%s\nwant:\n%s", got, want)
	}
}

func TestSyncRefusesToRemoveThroughASymlink(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	// .claude/skills/notes becomes a link to a directory outside the project, holding the same file.
	outside := t.TempDir()
	notes := filepath.Join(dir, ".claude/skills/notes")
	content, err := os.ReadFile(filepath.Join(notes, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(notes); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, notes); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, project.BaseDir, "skills/notes")); err != nil {
		t.Fatal(err)
	}

	if _, err := syncProject(dir, false); err == nil || !strings.Contains(err.Error(), ".claude/skills/notes is a symbolic link") {
		t.Errorf("Sync = %v, want an error naming the link", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "SKILL.md")); err != nil {
		t.Errorf("sync removed a file outside the project: %v", err)
	}
}

func TestSyncKeepsAStaleFileEditedByHand(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(dir, ".github/skills/notes/SKILL.md")
	if err := os.WriteFile(edited, []byte("edited by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, project.BaseDir, "skills/notes")); err != nil {
		t.Fatal(err)
	}

	report, err := syncProject(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.Kept, []string{".github/skills/notes/SKILL.md"}) {
		t.Errorf("kept %v, want the hand-edited notes skill", report.Kept)
	}
	if _, err := os.Stat(edited); err != nil {
		t.Errorf("sync removed a hand-edited file: %v", err)
	}
	if len(report.Removed) != 2 {
		t.Errorf("removed %v, want the two notes skills nobody edited", report.Removed)
	}

	// Every later check names it, until the developer removes it.
	differences, err := checkProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(differences, []string{".github/skills/notes/SKILL.md: no longer generated, but edited by hand; remove it yourself"}) {
		t.Errorf("Check after sync = %q, want the kept file named", differences)
	}
	if err := os.Remove(edited); err != nil {
		t.Fatal(err)
	}
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	if differences, err := checkProject(dir); err != nil || len(differences) > 0 {
		t.Errorf("Check once it is removed = (%q, %v), want nothing", differences, err)
	}
}

func TestCheckNamesStaleFilesAndTheLock(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, project.BaseDir, "skills/notes")); err != nil {
		t.Fatal(err)
	}
	differences, err := checkProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".claude/skills/notes/SKILL.md: no longer generated",
		".github/skills/notes/SKILL.md: no longer generated",
		".opencode/skills/notes/SKILL.md: no longer generated",
		".wf.lock: out of date",
	}
	if len(differences) != len(want) {
		t.Fatalf("Check = %q, want %d lines", differences, len(want))
	}
	for i, line := range want {
		if !strings.HasPrefix(differences[i], line) {
			t.Errorf("Check line %d = %q, want it to start %q", i, differences[i], line)
		}
	}

	if err := os.Remove(filepath.Join(dir, project.LockFile)); err != nil {
		t.Fatal(err)
	}
	differences, err = checkProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(differences, ".wf.lock: missing") {
		t.Errorf("Check with no lock = %q, want .wf.lock named as missing", differences)
	}
}

func TestSyncReadsWindowsLineEndings(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}
	// Inputs, generated files and the lock alike, as a checkout with core.autocrlf has them.
	for _, input := range []string{
		project.ConfigFile, ".agents/project/skills/naming/SKILL.md", ".agents/base/AGENTS.md",
		".claude/skills/hello/SKILL.md", "AGENTS.md", project.LockFile,
	} {
		full := filepath.Join(dir, filepath.FromSlash(input))
		content, err := os.ReadFile(full)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, bytes.ReplaceAll(content, []byte("\n"), []byte("\r\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if differences, err := checkProject(dir); err != nil || len(differences) > 0 {
		t.Errorf("Check after CRLF files = (%q, %v), want the same as with LF", differences, err)
	}
	report, err := syncProject(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Replaced) > 0 {
		t.Errorf("sync after CRLF files replaced %v, want nothing taken for a hand edit", report.Replaced)
	}

	// A byte order mark, as some Windows editors save, hides nothing either.
	naming := filepath.Join(dir, ".agents/project/skills/naming/SKILL.md")
	content, err := os.ReadFile(naming)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(naming, append([]byte("\xef\xbb\xbf"), content...), 0o644); err != nil {
		t.Fatal(err)
	}
	if differences, err := checkProject(dir); err != nil || len(differences) > 0 {
		t.Errorf("Check after a byte order mark = (%q, %v), want nothing", differences, err)
	}
}

func TestSyncKeepsAProjectPartsShape(t *testing.T) {
	dir := sample(t)
	if err := os.MkdirAll(filepath.Join(dir, ".agents/project/skills/notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{
		// Frontmatter holding only a comment overrides nothing.
		".agents/project/skills/notes/SKILL.md": "---\n# nothing yet\n---\nExtra.\n",
		// Empty frontmatter is no frontmatter.
		".agents/project/skills/naming/SKILL.md": "---\n---\nTables are plural.\n",
		// A part opening with an indented code block keeps its indentation.
		".agents/project/skills/hello/SKILL.md": "\n    go test ./...\n",
	}
	for path, content := range parts {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}

	notes, _ := os.ReadFile(filepath.Join(dir, ".claude/skills/notes/SKILL.md"))
	if !strings.Contains(string(notes), "description: Where notes go.") || !strings.HasSuffix(string(notes), "Extra.\n") {
		t.Errorf("notes with a comment-only frontmatter:\n%s", notes)
	}
	hello, _ := os.ReadFile(filepath.Join(dir, ".claude/skills/hello/SKILL.md"))
	if !strings.HasSuffix(string(hello), "override the base rules above.\n\n    go test ./...\n") {
		t.Errorf("hello lost its indentation:\n%s", hello)
	}
}

func TestSyncRefusesABrokenSource(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    string
	}{
		{
			name:    "a file wf does not know",
			path:    ".agents/base/skills/hello/notes.txt",
			content: "stray",
			want:    ".agents/base/skills/hello/notes.txt",
		},
		{
			name:    "a project-only skill with no description",
			path:    ".agents/project/skills/bare/SKILL.md",
			content: "---\nname: bare\n---\nNo description.\n",
			want:    `needs "description"`,
		},
		{
			name:    "a subagent with a key besides its description",
			path:    ".agents/base/subagents/reviewer.md",
			content: "---\ndescription: Reviews.\nmodel: fast\n---\nReview.\n",
			want:    `not "model"`,
		},
		{
			name:    "a template naming a key .wf.yml lacks",
			path:    ".agents/base/workflow/GUIDE.md",
			content: "[[ .missing ]]\n",
			want:    "missing",
		},
		{
			name:    "a skill with an empty description",
			path:    ".agents/project/skills/naming/SKILL.md",
			content: "---\ndescription: \"\"\n---\nRules.\n",
			want:    "description is empty",
		},
		{
			name:    "a frontmatter key written twice",
			path:    ".agents/project/skills/naming/SKILL.md",
			content: "---\ndescription: one\ndescription: two\n---\nRules.\n",
			want:    `"description" twice`,
		},
		{
			name:    "a skill named apart from its folder",
			path:    ".agents/project/skills/naming/SKILL.md",
			content: "---\nname: renamed\n---\nRules.\n",
			want:    `its folder is "naming"`,
		},
		{
			name:    "a project part for a template",
			path:    ".agents/project/workflow/SPEC-TEMPLATE.md",
			content: "Pinned elsewhere: the manifest.\n",
			want:    "a template takes no project part",
		},
		{
			name:    "frontmatter never closed",
			path:    ".agents/base/commands/review.md",
			content: "---\ndescription: Reviews.\n",
			want:    "never closed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := sample(t)
			full := filepath.Join(dir, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := syncProject(dir, false); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("Sync = %v, want an error containing %q", err, test.want)
			}
		})
	}
}

func TestSyncNeedsTheConfig(t *testing.T) {
	if _, err := syncProject(t.TempDir(), false); err == nil || !strings.Contains(err.Error(), project.ConfigFile) {
		t.Errorf("Sync = %v, want an error naming %s", err, project.ConfigFile)
	}
}

func TestSkill(t *testing.T) {
	dir := sample(t)
	if _, err := syncProject(dir, false); err != nil {
		t.Fatal(err)
	}

	shown, err := showSkill(dir, "hello", false)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(dir, ".claude/skills/hello/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shown, written) {
		t.Errorf("Skill(hello) differs from the file sync wrote:\n%s", shown)
	}

	base, err := showSkill(dir, "hello", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(base), "Greets sample's developers.") || strings.Contains(string(base), "English only") {
		t.Errorf("Skill(hello, base) is not the rendered base alone:\n%s", base)
	}

	if _, err := showSkill(dir, "local", true); err == nil {
		t.Error("Skill(local, base) returned a base for a project-only skill")
	}
	if _, err := showSkill(dir, "absent", false); err == nil {
		t.Error("Skill(absent) found a skill that does not exist")
	}
}

// syncProject syncs the project at dir with its own settings, as wf sync does.
func syncProject(dir string, force bool) (render.Report, error) {
	config, err := project.Load(dir)
	if err != nil {
		return render.Report{}, err
	}
	return render.Sync(dir, config, nil, version, force)
}

// checkProject compares the project at dir with what a sync would write, as wf sync --check does.
func checkProject(dir string) ([]string, error) {
	config, err := project.Load(dir)
	if err != nil {
		return nil, err
	}
	return render.Check(dir, config, nil, version)
}

// showSkill returns a skill of the project at dir, as wf show does.
func showSkill(dir, name string, baseOnly bool) ([]byte, error) {
	config, err := project.Load(dir)
	if err != nil {
		return nil, err
	}
	return render.Skill(dir, config, nil, version, name, baseOnly)
}

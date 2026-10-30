package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/worktree"
)

// repository returns a clone of a new repository whose origin has main, with .wf.yml asking for
// .env files to be copied, and the gitignored files that asks about.
func repository(t *testing.T) string {
	t.Helper()
	// Nothing of the developer's own git configuration takes part.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "wf")
	t.Setenv("GIT_AUTHOR_EMAIL", "wf@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "wf")
	t.Setenv("GIT_COMMITTER_EMAIL", "wf@example.com")

	base := t.TempDir()
	origin, dir := filepath.Join(base, "origin.git"), filepath.Join(base, "project")
	run(t, base, "init", "--quiet", "--bare", "--initial-branch=main", origin)
	run(t, base, "init", "--quiet", "--initial-branch=main", dir)
	files := map[string]string{
		".gitignore":       "/.claude/worktrees\n.env*\n/cache\n",
		".wf.yml":          "wf: 0.1.0\nforge: gitlab\nworktree:\n  copy: [\".env*\", cache, notes.txt]\n  skip: [\"*.example\"]\n",
		"README.md":        "a project\n",
		".env":             "SECRET=1\n",
		".env.example":     "SECRET=\n",
		"cache/blob":       "cached\n",
		"cache/deep/inner": "deeper\n",
		"notes.txt":        "untracked, not ignored\n",
	}
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, dir, "add", ".gitignore", ".wf.yml", "README.md")
	run(t, dir, "commit", "--quiet", "-m", "Start")
	run(t, dir, "remote", "add", "origin", origin)
	run(t, dir, "push", "--quiet", "origin", "main")
	// Paths git prints are resolved, as on macOS, where the temporary directory is a link.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestOpenCreatesResumesAndIsRemovedCleanly(t *testing.T) {
	dir := repository(t)
	want := filepath.Join(dir, ".claude/worktrees/12-sound-toggle")

	result, err := worktree.Open(dir, "12", "sound-toggle")
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != want {
		t.Fatalf("Open = %q, want %q", result.Path, want)
	}
	if branch := run(t, want, "branch", "--show-current"); branch != "12-sound-toggle" {
		t.Errorf("the worktree is on %q, want 12-sound-toggle", branch)
	}
	for name, content := range map[string]string{".env": "SECRET=1\n", "cache/deep/inner": "deeper\n"} {
		if got, err := os.ReadFile(filepath.Join(want, name)); err != nil || string(got) != content {
			t.Errorf("%s in the worktree = (%q, %v), want it copied", name, got, err)
		}
	}
	for _, name := range []string{".env.example", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(want, name)); err == nil {
			t.Errorf("%s was copied, but it is skipped or not gitignored", name)
		}
	}
	if len(result.Notes) != 1 || !strings.Contains(result.Notes[0], "notes.txt was not copied") {
		t.Errorf("notes = %q, want one naming notes.txt as not copied", result.Notes)
	}

	// Resumed under another slug, and from inside the worktree itself.
	for _, from := range []string{dir, want} {
		again, err := worktree.Open(from, "12", "another-slug")
		if err != nil || again.Path != want || !strings.Contains(strings.Join(again.Notes, " "), "already has a worktree") {
			t.Errorf("Open from %s again = (%+v, %v), want the same worktree, said to exist", from, again, err)
		}
	}

	// What was copied is gitignored, so the worktree is clean and removed without --force.
	run(t, dir, "worktree", "remove", want)

	// The branch is left; the next Open puts a worktree back on it.
	result, err = worktree.Open(dir, "12", "whatever")
	if err != nil || result.Path != want {
		t.Errorf("Open on a branch with no checkout = (%+v, %v), want a worktree at %s", result, err, want)
	}
}

func TestOpenFindsAPlanByItsExactBranch(t *testing.T) {
	dir := repository(t)
	run(t, dir, "branch", "plan-billing-v2", "main")
	result, err := worktree.Open(dir, "plan", "billing")
	if err != nil || result.Path != filepath.Join(dir, ".claude/worktrees/plan-billing") {
		t.Fatalf("Open(plan, billing) = (%+v, %v), want a new worktree on plan-billing", result, err)
	}
	if branch := run(t, result.Path, "branch", "--show-current"); branch != "plan-billing" {
		t.Errorf("the plan's worktree is on %q, want plan-billing", branch)
	}
}

func TestOpenRefusesToGuess(t *testing.T) {
	dir := repository(t)
	run(t, dir, "branch", "13-one", "main")
	run(t, dir, "branch", "13-two", "main")
	if _, err := worktree.Open(dir, "13", "one"); err == nil || !strings.Contains(err.Error(), "13-one\n13-two") {
		t.Errorf("Open with two branches of an Issue = %v, want an error listing both", err)
	}
}

func TestOpenSaysWhereTheIssueIsCheckedOut(t *testing.T) {
	dir := repository(t)
	run(t, dir, "switch", "--quiet", "-c", "14-in-place")
	result, err := worktree.Open(dir, "14", "in-place")
	if err != nil || result.Path != dir || !strings.Contains(strings.Join(result.Notes, " "), "main checkout itself") {
		t.Errorf("Open on the Issue the main checkout is on = (%+v, %v), want the main checkout", result, err)
	}
}

func TestOpenRemovesAWorktreeItCouldNotFill(t *testing.T) {
	dir := repository(t)
	unreadable := filepath.Join(dir, "cache/blob")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o644) })
	if _, err := os.ReadFile(unreadable); err == nil {
		t.Skip("this user reads any file, so a copy cannot be made to fail")
	}

	if _, err := worktree.Open(dir, "15", "broken"); err == nil || !strings.Contains(err.Error(), "removed again") {
		t.Fatalf("Open with a copy that fails = %v, want an error saying the worktree was removed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude/worktrees/15-broken")); err == nil {
		t.Error("the half-made worktree is still there")
	}
	if branches := run(t, dir, "branch", "--list", "15-*"); branches != "" {
		t.Errorf("the half-made worktree's branch is still there: %q", branches)
	}
}

func TestOpenSaysWhenWorktreesAreNotIgnored(t *testing.T) {
	dir := repository(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env*\n/cache\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := worktree.Open(dir, "16", "noisy")
	if err != nil || !strings.Contains(strings.Join(result.Notes, " "), "Add /.claude/worktrees to .gitignore") {
		t.Errorf("Open with worktrees not ignored = (%+v, %v), want a note saying to ignore them", result, err)
	}
}

func TestOpenOutsideARepository(t *testing.T) {
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	if _, err := worktree.Open(t.TempDir(), "1", "anything"); err == nil {
		t.Error("Open outside a repository succeeded")
	}
}

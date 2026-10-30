// Package worktree gives an Issue, or a plan, a git worktree of its own, so several can be worked
// on at once while the main checkout stays on main.
//
// Worktrees live in .claude/worktrees/, inside the repository: Codex and Copilot write only inside
// the folder they were opened in, and Claude Code enters a worktree under .claude/worktrees/
// without asking each time.
package worktree

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/workflowaiorg/workflow/internal/render"
)

// Dir is where worktrees are made, relative to the main checkout.
const Dir = ".claude/worktrees"

// Result is where the work goes on, and anything the developer should know about it.
type Result struct {
	Path  string
	Notes []string
}

// Open returns the worktree of an Issue, or of a plan when issue is "plan", making it if there is
// none. dir is anywhere in the repository, a worktree included: paths are taken from the main
// checkout, so a worktree is never nested in another.
//
// An Issue owns every branch <issue>-*, whatever its slug, and is found by it wherever it is
// checked out; a plan owns exactly plan-<slug>. A branch with no checkout gets a worktree, and an
// Issue or plan with no branch gets <issue>-<slug> from origin/main. The gitignored files .wf.yml
// lists under worktree: are copied in from the main checkout, and a worktree whose copy failed is
// removed again rather than handed over half-made.
func Open(dir, issue, slug string) (Result, error) {
	common, err := git(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Result{}, err
	}
	root := filepath.Dir(common)

	what, owned, exact := "Issue #"+issue, issue+"-", false
	if issue == "plan" {
		what, owned, exact = "Plan "+slug, "plan-"+slug, true
	}
	owns := func(branch string) bool {
		if exact {
			return branch == owned
		}
		return strings.HasPrefix(branch, owned)
	}

	if _, err := git(root, "worktree", "prune"); err != nil {
		return Result{}, err
	}
	list, err := git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return Result{}, err
	}
	// Porcelain lists "worktree <path>" and then "branch refs/heads/<name>" for each checkout.
	var existing []string
	var current string
	for line := range strings.Lines(list) {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "worktree "); ok {
			current = after
		}
		if branch, ok := strings.CutPrefix(line, "branch refs/heads/"); ok && owns(branch) {
			existing = append(existing, current)
		}
	}
	switch len(existing) {
	case 0:
	case 1:
		result := Result{Path: existing[0], Notes: []string{what + " already has a worktree."}}
		switch {
		case existing[0] == root:
			result.Notes = append(result.Notes, "It is checked out in the main checkout itself: work there, in place.")
		case !strings.HasPrefix(existing[0], filepath.Join(root, Dir)+string(filepath.Separator)):
			result.Notes = append(result.Notes, "It is not under "+Dir+"/, where Claude Code asks before entering.")
		}
		return result, nil
	default:
		return Result{}, fmt.Errorf("%s is checked out in more than one worktree; say which one to continue:\n%s", what, strings.Join(existing, "\n"))
	}

	refs, err := git(root, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return Result{}, err
	}
	var branches []string
	for branch := range strings.Lines(refs) {
		if branch = strings.TrimSpace(branch); owns(branch) {
			branches = append(branches, branch)
		}
	}

	var worktree string
	var undo func() error
	switch len(branches) {
	case 0:
		branch := issue + "-" + slug
		worktree = filepath.Join(root, Dir, branch)
		if _, err := git(root, "fetch", "--quiet", "origin", "main"); err != nil {
			return Result{}, err
		}
		if _, err := git(root, "worktree", "add", "--quiet", "--no-track", "-b", branch, worktree, "origin/main"); err != nil {
			return Result{}, err
		}
		undo = func() error {
			_, removeErr := git(root, "worktree", "remove", "--force", worktree)
			_, branchErr := git(root, "branch", "-D", branch)
			return errors.Join(removeErr, branchErr)
		}
	case 1:
		worktree = filepath.Join(root, Dir, branches[0])
		if _, err := git(root, "worktree", "add", "--quiet", worktree, branches[0]); err != nil {
			return Result{}, err
		}
		undo = func() error {
			_, err := git(root, "worktree", "remove", "--force", worktree)
			return err
		}
	default:
		return Result{}, fmt.Errorf("%s has more than one branch; say which one to continue:\n%s", what, strings.Join(branches, "\n"))
	}

	notes, err := copyIgnored(root, worktree)
	if err != nil {
		if undoErr := undo(); undoErr != nil {
			return Result{}, fmt.Errorf("copying into the new worktree failed: %w; removing it failed too, so %s is left half-made: %w", err, worktree, undoErr)
		}
		return Result{}, fmt.Errorf("copying into the new worktree failed, so it has been removed again: %w", err)
	}
	if _, err := git(root, "check-ignore", "--quiet", Dir+"/x"); err != nil {
		notes = append(notes, "Add /"+Dir+" to .gitignore, or git status in the main checkout lists every worktree.")
	}
	return Result{Path: worktree, Notes: notes}, nil
}

// config is the part of .wf.yml this package reads.
type config struct {
	Worktree struct {
		// Copy holds globs, from the root, of the gitignored files a new worktree needs.
		Copy []string `yaml:"copy"`
		// Skip holds globs of what Copy matches but must not be copied, such as "*.example". Each
		// is matched against the path from the root and against the file's name.
		Skip []string `yaml:"skip"`
	} `yaml:"worktree"`
}

// copyIgnored copies from root into worktree what .wf.yml lists under worktree: and git ignores.
// What git does not ignore is not copied: a tracked file is in the worktree already, and an
// untracked one would make it look changed.
func copyIgnored(root, worktree string) ([]string, error) {
	content, err := render.ReadText(filepath.Join(root, render.ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", render.ConfigFile, err)
	}
	var read config
	if err := yaml.Unmarshal(content, &read); err != nil {
		return nil, fmt.Errorf("reading %s: %w", render.ConfigFile, err)
	}

	var notes []string
	for _, pattern := range read.Worktree.Copy {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, fmt.Errorf("%s: worktree.copy has a bad pattern %q", render.ConfigFile, pattern)
		}
		for _, match := range matches {
			rel, err := filepath.Rel(root, match)
			if err != nil {
				return nil, fmt.Errorf("copying %s: %w", match, err)
			}
			rel = filepath.ToSlash(rel)
			if slices.ContainsFunc(read.Worktree.Skip, func(skip string) bool {
				byPath, _ := path.Match(skip, rel)
				byName, _ := path.Match(skip, path.Base(rel))
				return byPath || byName
			}) {
				continue
			}
			if _, err := git(root, "check-ignore", "--quiet", rel); err != nil {
				notes = append(notes, rel+" was not copied: git does not ignore it.")
				continue
			}
			if err := copyPath(match, filepath.Join(worktree, filepath.FromSlash(rel))); err != nil {
				return nil, fmt.Errorf("%s: %w", rel, err)
			}
		}
	}
	return notes, nil
}

// copyPath copies the file or directory at from to to.
func copyPath(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.CopyFS(to, os.DirFS(from))
	}
	content, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, content, info.Mode().Perm())
}

// git runs git in dir and returns what it printed, or an error carrying what it said.
func git(dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return strings.TrimSpace(stdout.String()), nil
}

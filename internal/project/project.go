// Package project is what wf knows of the project it works on: where its files are, its settings
// in .wf.yml, and how wf tells the developer's mistakes from its own faults.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The files and directories of a project that wf reads and writes, relative to its root.
const (
	ConfigFile = ".wf.yml"
	LockFile   = ".wf.lock"
	BaseDir    = ".agents/base"
	ProjectDir = ".agents/project"
)

// Forges are the forges wf has a base for.
var Forges = []string{"gitlab"}

// UserError is the developer's mistake rather than wf's fault: a setting that is wrong, or
// something they named that is not there. wf exits 2 on one.
type UserError struct {
	message string
	err     error
}

func (err *UserError) Error() string { return err.message }

func (err *UserError) Unwrap() error { return err.err }

// UserErrorf returns a UserError saying what format and args say.
func UserErrorf(format string, args ...any) error {
	return &UserError{message: fmt.Sprintf(format, args...)}
}

// Config is a project's .wf.yml.
type Config struct {
	// WF is the version of wf the project pins.
	WF    string
	Forge string
	// Worktree is what wf worktree copies into a new worktree.
	Worktree Worktree
	// Values is the whole of .wf.yml as written: the data wf's base files are rendered with.
	Values map[string]any
}

// Worktree holds the gitignored files a new worktree needs.
type Worktree struct {
	// Copy holds globs, from the root, of the gitignored files a new worktree needs.
	Copy []string
	// Skip holds globs of what Copy matches but must not be copied, such as "*.example". Each is
	// matched against the path from the root and against the file's name.
	Skip []string
}

// Load reads and checks the .wf.yml of the project at dir. Every mistake in it comes back as one
// UserError naming them all, so none is found only later, by the command that happens to read it.
// A project with no .wf.yml gets a UserError that is also fs.ErrNotExist.
func Load(dir string) (Config, error) {
	content, err := ReadText(filepath.Join(dir, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, &UserError{fmt.Sprintf("no %s in %s: run wf in the project's root", ConfigFile, dir), err}
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	values := map[string]any{}
	if err := yaml.Unmarshal(content, &values); err != nil {
		return Config{}, UserErrorf("reading %s: %v", ConfigFile, err)
	}

	// The pin and the forge are read as written: as a number, wf: 1.10 would be 1.1.
	var named struct {
		WF    string `yaml:"wf"`
		Forge string `yaml:"forge"`
	}
	if err := yaml.Unmarshal(content, &named); err != nil {
		return Config{}, UserErrorf("reading %s: %v", ConfigFile, err)
	}

	config := Config{WF: named.WF, Values: values}
	var problems []string
	if named.WF == "" {
		problems = append(problems, "pins no wf version; add wf: and the version wf version prints, such as wf: 0.1.0")
	}
	switch {
	case named.Forge == "":
		problems = append(problems, fmt.Sprintf("names no forge; add forge: %s", strings.Join(Forges, " or ")))
	case !slices.Contains(Forges, named.Forge):
		problems = append(problems, fmt.Sprintf("forge: %s is not one wf has a base for; it knows %s", named.Forge, strings.Join(Forges, ", ")))
	default:
		config.Forge = named.Forge
	}
	problems = append(problems, gateProblems(values["gate"])...)
	worktree, worktreeProblems := readWorktree(values["worktree"])
	config.Worktree = worktree
	problems = append(problems, worktreeProblems...)
	if len(problems) > 0 {
		return Config{}, UserErrorf("%s: %s", ConfigFile, strings.Join(problems, "; "))
	}
	return config, nil
}

// gateProblems says what is wrong with gate:, the project's own steps for /pre-mr-check: a list
// of entries, each with a name, a list of paths and the command to run.
func gateProblems(gate any) []string {
	if gate == nil {
		return nil
	}
	steps, ok := gate.([]any)
	if !ok {
		return []string{"gate: is a list of steps, each with name:, paths: and run:"}
	}
	var problems []string
	for i, step := range steps {
		fields, _ := step.(map[string]any)
		name, _ := fields["name"].(string)
		where := fmt.Sprintf("gate[%d]", i)
		if name != "" {
			where += " (" + name + ")"
		}
		var missing []string
		if name == "" {
			missing = append(missing, "name: the step's name")
		}
		if paths, ok := stringList(fields["paths"]); !ok || len(paths) == 0 {
			missing = append(missing, `paths: a list of globs, such as ["**/*.go"]`)
		}
		if run, _ := fields["run"].(string); run == "" {
			missing = append(missing, "run: the command")
		}
		if len(missing) > 0 {
			problems = append(problems, where+" needs "+strings.Join(missing, ", "))
		}
	}
	return problems
}

// readWorktree reads worktree:, saying what is wrong with it.
func readWorktree(value any) (Worktree, []string) {
	if value == nil {
		return Worktree{}, nil
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return Worktree{}, []string{"worktree: holds copy: and skip:, each a list of globs"}
	}
	var worktree Worktree
	var problems []string
	for _, key := range []string{"copy", "skip"} {
		if fields[key] == nil {
			continue
		}
		globs, ok := stringList(fields[key])
		if !ok {
			problems = append(problems, fmt.Sprintf(`worktree.%s is a list of globs, such as [".env*"]`, key))
			continue
		}
		for _, glob := range globs {
			if _, err := path.Match(glob, ""); err != nil {
				problems = append(problems, fmt.Sprintf("worktree.%s has a bad pattern %q", key, glob))
			}
		}
		if key == "copy" {
			worktree.Copy = globs
		} else {
			worktree.Skip = globs
		}
	}
	return worktree, problems
}

// stringList returns value as a list of strings, and whether it is one with none of them empty.
func stringList(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	list := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok || text == "" {
			return nil, false
		}
		list = append(list, text)
	}
	return list, true
}

// ReadText reads a text file as wf compares and renders it: Windows line endings become \n and a
// byte order mark is dropped, so a checkout with core.autocrlf, or a file saved by a Windows
// editor, reads the same as any other.
func ReadText(file string) ([]byte, error) {
	content, err := os.ReadFile(file)
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n")), err
}

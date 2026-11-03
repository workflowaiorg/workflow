// Package base is what wf carries for every project: the base it installs into .agents/base,
// assembled with the parts of the project's components, the files it gives a project once, and the
// check that a project's .agents/base is still the base of the version it pins.
package base

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/project"
)

// files holds core/, the base every project gets; forge-<name>/, the base a forge adds; and
// skeleton/, files a new project is given once and then owns.
//
//go:embed all:files
var files embed.FS

// sections are the files a component adds to as a section of its own, rather than owning alone.
var sections = []string{"AGENTS.md", "workflow/GUIDE.md"}

// Files returns the base of a project on forge with components installed, by its slash-separated
// path under .agents/base: wf's core, the forge's part, then each component's.
func Files(forge string, components []component.Component) (map[string][]byte, error) {
	if !slices.Contains(project.Forges, forge) {
		return nil, fmt.Errorf("wf has no base for the forge %q; it knows %s", forge, strings.Join(project.Forges, ", "))
	}
	base := map[string][]byte{}
	for _, root := range []string{"files/core", "files/forge-" + forge} {
		if err := read(files, root, base); err != nil {
			return nil, err
		}
	}
	// Each part of a component, its own and its chosen options', adds to the base as a component does.
	for _, c := range components {
		for _, p := range c.Parts() {
			if p.Base == nil {
				continue
			}
			part := map[string][]byte{}
			if err := read(p.Base, ".", part); err != nil {
				return nil, fmt.Errorf("reading %s: %w", c, err)
			}
			if err := add(base, part, c); err != nil {
				return nil, err
			}
		}
	}
	return base, nil
}

// add adds part, a part of c's base, to base: a section file to the one already there, any other
// file only when no other part has it.
func add(base, part map[string][]byte, c component.Component) error {
	for _, name := range slices.Sorted(maps.Keys(part)) {
		existing, taken := base[name]
		switch {
		case slices.Contains(sections, name) && taken:
			base[name] = slices.Concat(bytes.TrimRight(existing, "\n"), []byte("\n\n"), part[name])
		case taken:
			// Two sources of one file would leave one of them silently unused.
			return fmt.Errorf("%s and another part of the base both have %s", c, name)
		default:
			base[name] = part[name]
		}
	}
	return nil
}

// Install writes the base of a project on forge with components installed into dir's
// .agents/base, replacing whatever was there: .agents/base is wf's alone.
func Install(dir, forge string, components []component.Component) error {
	base, err := Files(forge, components)
	if err != nil {
		return err
	}
	root := filepath.Join(dir, filepath.FromSlash(project.BaseDir))
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("replacing %s: %w", project.BaseDir, err)
	}
	for _, name := range slices.Sorted(maps.Keys(base)) {
		if err := write(filepath.Join(root, filepath.FromSlash(name)), base[name]); err != nil {
			return fmt.Errorf("writing %s/%s: %w", project.BaseDir, name, err)
		}
	}
	return nil
}

// Skeleton writes into dir each file a new project starts with that is not there yet, and returns
// the paths it wrote. They are the project's from then on, and wf never writes them again.
func Skeleton(dir string) ([]string, error) {
	return writeOnce(dir, files, "files/skeleton")
}

// ComponentSkeleton writes into dir each file c, added with answers, gives a project that is not
// there yet, and returns the paths it wrote. Like the project's own skeleton, they are the
// project's from then on. A file named *.tmpl is a template, rendered with answers and written
// without its .tmpl: Go source is kept as one, so wf's own build never compiles a project's code.
func ComponentSkeleton(dir string, c component.Component, answers map[string]any) ([]string, error) {
	skeleton := map[string][]byte{}
	for _, part := range c.With(answers).Parts() {
		if part.Skeleton == nil {
			continue
		}
		if err := readOnce(part.Skeleton, skeleton); err != nil {
			return nil, fmt.Errorf("reading %s: %w", c, err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(skeleton)) {
		target, isTemplate := strings.CutSuffix(name, ".tmpl")
		if !isTemplate {
			continue
		}
		tmpl, err := template.New(name).Delims("[[", "]]").Option("missingkey=error").Parse(string(skeleton[name]))
		if err != nil {
			return nil, fmt.Errorf("reading %s's %s: %w", c, name, err)
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, answers); err != nil {
			return nil, fmt.Errorf("rendering %s's %s: %w", c, name, err)
		}
		delete(skeleton, name)
		skeleton[target] = out.Bytes()
	}
	return writeFiles(dir, skeleton)
}

func writeOnce(dir string, fsys fs.FS, root string) ([]string, error) {
	skeleton := map[string][]byte{}
	if err := read(fsys, root, skeleton); err != nil {
		return nil, err
	}
	return writeFiles(dir, skeleton)
}

// writeFiles writes each of skeleton's files into dir that is not there yet, and returns the paths
// it wrote.
func writeFiles(dir string, skeleton map[string][]byte) ([]string, error) {
	var written []string
	for _, name := range slices.Sorted(maps.Keys(skeleton)) {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if _, err := os.Lstat(full); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return written, fmt.Errorf("checking %s: %w", name, err)
		}
		if err := write(full, skeleton[name]); err != nil {
			return written, fmt.Errorf("writing %s: %w", name, err)
		}
		written = append(written, name)
	}
	return written, nil
}

// Check compares dir's .agents/base with the base this version of wf carries for the project's
// forge and its components, and writes nothing. It returns one line per file that differs; none
// means it is the same.
func Check(dir string, config project.Config, components []component.Component, version string) ([]string, error) {
	if config.WF != version {
		// Only the pinned version carries the base to compare with.
		return []string{fmt.Sprintf("%s: pins wf %s, but this is wf %s; run the version it pins", project.ConfigFile, config.WF, version)}, nil
	}
	want, err := Files(config.Forge, components)
	if err != nil {
		return nil, err
	}
	have, err := readBase(dir)
	if err != nil {
		return nil, err
	}

	var differences []string
	for _, name := range slices.Sorted(maps.Keys(want)) {
		content, ok := have[name]
		switch {
		case !ok:
			differences = append(differences, fmt.Sprintf("%s/%s: missing from wf %s's base", project.BaseDir, name, version))
		case string(content) != string(want[name]):
			differences = append(differences, fmt.Sprintf("%s/%s: differs from wf %s's base; it is never edited — put the change in %s/%s and restore this file", project.BaseDir, name, version, project.ProjectDir, name))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(have)) {
		if _, ok := want[name]; !ok {
			differences = append(differences, fmt.Sprintf("%s/%s: not part of wf %s's base; the project's own files go in %s", project.BaseDir, name, version, project.ProjectDir))
		}
	}
	return differences, nil
}

// Changed returns the path under .agents/base of each file that installing the base of a project
// on forge with components would add, change or remove in dir, and writes nothing.
func Changed(dir, forge string, components []component.Component) ([]string, error) {
	want, err := Files(forge, components)
	if err != nil {
		return nil, err
	}
	have, err := readBase(dir)
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if content, ok := have[name]; !ok || string(content) != string(want[name]) {
			changed = append(changed, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(have)) {
		if _, ok := want[name]; !ok {
			changed = append(changed, name)
		}
	}
	slices.Sort(changed)
	return changed, nil
}

// readBase reads dir's .agents/base, by slash-separated path under it. A missing one has no files.
func readBase(dir string) (map[string][]byte, error) {
	root := filepath.Join(dir, filepath.FromSlash(project.BaseDir))
	have := map[string][]byte{}
	err := filepath.WalkDir(root, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".") && full != root {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		content, err := project.ReadText(full)
		if err != nil {
			return err
		}
		have[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading %s: %w", project.BaseDir, err)
	}
	return have, nil
}

// readOnce adds every file in fsys to into, failing on one into has already: two parts of one
// component with the same file would leave one of them silently unused.
func readOnce(fsys fs.FS, into map[string][]byte) error {
	part := map[string][]byte{}
	if err := read(fsys, ".", part); err != nil {
		return err
	}
	for name, content := range part {
		if _, taken := into[name]; taken {
			return fmt.Errorf("two of its parts both have %s", name)
		}
		into[name] = content
	}
	return nil
}

// read adds every file under root in fsys to into, by its path under root.
func read(fsys fs.FS, root string, into map[string][]byte) error {
	return fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		if root != "." {
			name = strings.TrimPrefix(name, root+"/")
		}
		into[name] = content
		return nil
	})
}

func write(full string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, content, 0o644)
}

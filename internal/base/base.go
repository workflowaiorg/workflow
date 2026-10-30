// Package base is what wf carries for every project: the base it installs into .agents/base, the
// files it gives a new project once, and the check that a project's .agents/base is still the base
// of the version it pins.
package base

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/project"
)

// files holds core/, the base every project gets; forge-<name>/, the base a forge adds; and
// skeleton/, files a new project is given once and then owns.
//
//go:embed all:files
var files embed.FS

// Files returns the base of a project on forge, by its slash-separated path under .agents/base.
func Files(forge string) (map[string][]byte, error) {
	if !slices.Contains(project.Forges, forge) {
		return nil, fmt.Errorf("wf has no base for the forge %q; it knows %s", forge, strings.Join(project.Forges, ", "))
	}
	base := map[string][]byte{}
	for _, root := range []string{"files/core", "files/forge-" + forge} {
		if err := read(root, base); err != nil {
			return nil, err
		}
	}
	return base, nil
}

// Install writes the base of a project on forge into dir's .agents/base, replacing whatever was
// there: .agents/base is wf's alone.
func Install(dir, forge string) error {
	base, err := Files(forge)
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
	skeleton := map[string][]byte{}
	if err := read("files/skeleton", skeleton); err != nil {
		return nil, err
	}
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
// forge, and writes nothing. It returns one line per file that differs; none means it is the same.
func Check(dir string, config project.Config, version string) ([]string, error) {
	if config.WF != version {
		// Only the pinned version carries the base to compare with.
		return []string{fmt.Sprintf("%s: pins wf %s, but this is wf %s; run the version it pins", project.ConfigFile, config.WF, version)}, nil
	}
	want, err := Files(config.Forge)
	if err != nil {
		return nil, err
	}

	root := filepath.Join(dir, filepath.FromSlash(project.BaseDir))
	have := map[string][]byte{}
	err = filepath.WalkDir(root, func(full string, entry fs.DirEntry, err error) error {
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

// read adds every file under root in the embedded files to into, by its path under root.
func read(root string, into map[string][]byte) error {
	return fs.WalkDir(files, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		into[strings.TrimPrefix(name, root+"/")] = content
		return nil
	})
}

func write(full string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, content, 0o644)
}

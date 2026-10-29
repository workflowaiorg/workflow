package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// lock is .wf.lock: the wf version that wrote the generated files, and a hash of each. It is how
// wf tells its own files from anyone else's.
type lock struct {
	WF    string            `yaml:"wf"`
	Files map[string]string `yaml:"files"`
}

const lockHeader = "# Written by wf sync. Do not edit.\n"

// Report says what Sync did, each list holding slash-separated paths from the project root.
type Report struct {
	Written []string
	// Replaced were overwritten although they were not what the last sync wrote: wf's own files
	// edited by hand, or, with force, files that were there before wf.
	Replaced []string
	// Skipped were there before wf and were left alone, because force was not given.
	Skipped []string
	Removed []string
	// Kept are no longer generated but were edited by hand, so they are left for the developer.
	Kept      []string
	Unchanged int
}

// plan is what a sync of the project would do, worked out without writing anything.
type plan struct {
	files    []File
	previous lock
	current  map[string][]byte // what is on disk now, by path; a missing file has no entry
	skipped  []string
	stale    []string
	kept     []string
	lock     []byte
}

// Sync writes every generated file of the project at dir and its .wf.lock, and removes the files
// an earlier sync generated that it no longer does. It never writes .agents/project. A file at a
// generated path that wf did not write is skipped unless force is set.
func Sync(dir, version string, force bool) (Report, error) {
	p, err := newPlan(dir, version, force)
	if err != nil {
		return Report{}, err
	}

	report := Report{Skipped: p.skipped, Kept: p.kept}
	for _, file := range p.files {
		current, exists := p.current[file.Path]
		switch {
		case slices.Contains(p.skipped, file.Path):
			continue
		case exists && bytes.Equal(current, file.Data):
			report.Unchanged++
			continue
		case exists && hash(current) != p.previous.Files[file.Path]:
			report.Replaced = append(report.Replaced, file.Path)
		default:
			report.Written = append(report.Written, file.Path)
		}
		if err := writeFile(dir, file.Path, file.Data); err != nil {
			return report, err
		}
	}

	for _, stale := range p.stale {
		full := filepath.Join(dir, filepath.FromSlash(stale))
		if err := os.Remove(full); err != nil {
			return report, fmt.Errorf("removing %s: %w", stale, err)
		}
		report.Removed = append(report.Removed, stale)
		removeEmptyParents(dir, filepath.Dir(full))
	}

	if current, err := readText(filepath.Join(dir, LockFile)); err != nil || !bytes.Equal(current, p.lock) {
		if err := writeFile(dir, LockFile, p.lock); err != nil {
			return report, err
		}
		report.Written = append(report.Written, LockFile)
	}
	return report, nil
}

// Check compares the project at dir with what Sync would write, and writes nothing. It returns one
// line per file that differs, naming it; none means the project is in sync.
func Check(dir, version string) ([]string, error) {
	p, err := newPlan(dir, version, false)
	if err != nil {
		return nil, err
	}

	var differences []string
	for _, file := range p.files {
		current, exists := p.current[file.Path]
		switch {
		case slices.Contains(p.skipped, file.Path):
			differences = append(differences, file.Path+": not written by wf; wf sync --force replaces it")
		case !exists:
			differences = append(differences, file.Path+": missing")
		case !bytes.Equal(current, file.Data):
			differences = append(differences, file.Path+": differs from what wf sync writes; it is never edited by hand")
		}
	}
	for _, stale := range p.stale {
		differences = append(differences, stale+": no longer generated; wf sync removes it")
	}
	for _, kept := range p.kept {
		differences = append(differences, kept+": no longer generated, but edited by hand; remove it yourself")
	}
	current, err := readText(filepath.Join(dir, LockFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		differences = append(differences, LockFile+": missing")
	case err != nil:
		return nil, fmt.Errorf("reading %s: %w", LockFile, err)
	case !bytes.Equal(current, p.lock):
		differences = append(differences, LockFile+": out of date")
	}
	return differences, nil
}

func newPlan(dir, version string, force bool) (plan, error) {
	files, err := Render(dir, version)
	if err != nil {
		return plan{}, err
	}
	previous, err := readLock(dir)
	if err != nil {
		return plan{}, err
	}
	p := plan{files: files, previous: previous, current: map[string][]byte{}}

	// Every path wf may write or remove is checked before anything is written, so a link that
	// would lead a write elsewhere stops the whole sync rather than half of it.
	paths := []string{LockFile}
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	for path := range previous.Files {
		paths = append(paths, path)
	}
	for _, path := range paths {
		if err := noSymlinks(dir, path); err != nil {
			return plan{}, err
		}
	}

	read := func(path string) error {
		content, err := readText(filepath.Join(dir, filepath.FromSlash(path)))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		p.current[path] = content
		return nil
	}

	next := lock{WF: version, Files: map[string]string{}}
	for _, file := range files {
		if err := read(file.Path); err != nil {
			return plan{}, err
		}
		current, exists := p.current[file.Path]
		_, owned := previous.Files[file.Path]
		// A file wf never wrote is the project's until the developer says otherwise.
		if exists && !owned && !force && !bytes.Equal(current, file.Data) {
			p.skipped = append(p.skipped, file.Path)
			continue
		}
		next.Files[file.Path] = hash(file.Data)
	}

	for path, recorded := range previous.Files {
		if slices.ContainsFunc(files, func(file File) bool { return file.Path == path }) {
			continue
		}
		if err := read(path); err != nil {
			return plan{}, err
		}
		current, exists := p.current[path]
		switch {
		case !exists:
		case hash(current) == recorded:
			p.stale = append(p.stale, path)
		default:
			// Still listed, so every later sync and check names it until the developer removes it.
			p.kept = append(p.kept, path)
			next.Files[path] = recorded
		}
	}
	slices.Sort(p.stale)
	slices.Sort(p.kept)

	encoded := bytes.NewBufferString(lockHeader)
	encoder := yaml.NewEncoder(encoded)
	encoder.SetIndent(2)
	if err := encoder.Encode(next); err != nil {
		return plan{}, fmt.Errorf("writing %s: %w", LockFile, err)
	}
	if err := encoder.Close(); err != nil {
		return plan{}, fmt.Errorf("writing %s: %w", LockFile, err)
	}
	p.lock = encoded.Bytes()
	return p, nil
}

func readLock(dir string) (lock, error) {
	content, err := readText(filepath.Join(dir, LockFile))
	if errors.Is(err, fs.ErrNotExist) {
		return lock{}, nil
	}
	if err != nil {
		return lock{}, fmt.Errorf("reading %s: %w", LockFile, err)
	}
	var read lock
	if err := yaml.Unmarshal(content, &read); err != nil {
		return lock{}, fmt.Errorf("reading %s: %w", LockFile, err)
	}
	// wf removes the files the lock lists, so a path wf could not have generated is refused rather
	// than trusted.
	for path := range read.Files {
		if !generatable(path) {
			return lock{}, fmt.Errorf("reading %s: %q is not a file wf generates", LockFile, path)
		}
	}
	return read, nil
}

// writeFile writes data to path under dir through a temporary file renamed into place, so a crash
// leaves the old file or the new one, never half of one.
func writeFile(dir, path string, data []byte) error {
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	temp, err := os.CreateTemp(filepath.Dir(full), ".wf-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(temp.Name(), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(temp.Name(), full); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// noSymlinks fails when the file at path, or any directory on the way to it from dir, is a
// symbolic link. wf never makes one, but a project may have, and a write through a link to
// .agents/project would overwrite the developer's own rules.
func noSymlinks(dir, path string) error {
	current := dir
	for _, part := range strings.Split(path, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("checking %s: %w", path, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			rel, _ := filepath.Rel(dir, current)
			return fmt.Errorf("%s is a symbolic link; wf writes no generated file through one, so replace it with a plain directory or file", filepath.ToSlash(rel))
		}
	}
	return nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// removeEmptyParents removes dir and each parent above it while they are empty, stopping at root.
func removeEmptyParents(root, dir string) {
	for dir != root && len(dir) > len(root) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

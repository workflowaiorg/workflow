package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/project"
)

// sumLine is one line of .wf.sum: a release file and its SHA-256. The file's name is not checked
// further: a later wf may record a platform, or a kind of archive, this one does not know, and
// this one still reads its own line from the file.
var sumLine = regexp.MustCompile(`^([^\s/]+) (sha256:[0-9a-f]{64})$`)

// sums is .wf.sum: each archive of the pinned release, with its SHA-256 as "sha256:<hex>".
type sums map[string]string

// readSum reads the .wf.sum of the project at dir. A project with none has no sums.
func readSum(dir string) (sums, error) {
	content, err := project.ReadText(filepath.Join(dir, project.SumFile))
	if errors.Is(err, fs.ErrNotExist) {
		return sums{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", project.SumFile, err)
	}
	read := sums{}
	if len(content) == 0 {
		return read, nil
	}
	for number, line := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
		match := sumLine.FindStringSubmatch(line)
		if match == nil {
			return nil, project.UserErrorf("%s: line %d is not <archive> sha256:<64 hex digits>; wf lock writes it, so restore it from git and run wf lock", project.SumFile, number+1)
		}
		if _, ok := read[match[1]]; ok {
			return nil, project.UserErrorf("%s: %s is listed twice; restore it from git and run wf lock", project.SumFile, match[1])
		}
		read[match[1]] = match[2]
	}
	return read, nil
}

// encode returns s as .wf.sum holds it: sorted, one archive a line.
func (s sums) encode() []byte {
	var out strings.Builder
	for _, archive := range slices.Sorted(maps.Keys(s)) {
		fmt.Fprintf(&out, "%s %s\n", archive, s[archive])
	}
	return []byte(out.String())
}

// LockResult says what Lock did.
type LockResult struct {
	// Version is the version pinned.
	Version string
	// Changed is whether .wf.sum was written.
	Changed bool
}

// Lock records in the project's .wf.sum the SHA-256 of every archive of the release it pins, from
// the release's checksums.txt. It goes to the network only when one is missing, and never replaces
// a fingerprint already recorded with a different one: that is what the file protects against.
func (l *Launcher) Lock() (LockResult, error) {
	pin, exists, err := readPin(l.Dir)
	switch {
	case err != nil:
		return LockResult{}, err
	case !exists:
		return LockResult{}, project.UserErrorf("no %s in %s: run wf in the project's root", project.ConfigFile, l.Dir)
	case pin == "":
		return LockResult{}, project.UserErrorf("%s pins no wf version; add wf: and the version wf version prints, such as wf: 0.1.0", project.ConfigFile)
	}
	if err := checkVersion(pin); err != nil {
		return LockResult{}, err
	}
	recorded, err := readSum(l.Dir)
	if err != nil {
		return LockResult{}, err
	}

	// The pin's lines are kept, those of platforms this wf does not know included: a later wf
	// recorded them.
	next := sums{}
	for archive, sum := range recorded {
		if strings.HasPrefix(archive, "wf_"+pin+"_") {
			next[archive] = sum
		}
	}
	complete := true
	for _, platform := range Platforms {
		if _, ok := next[Archive(pin, platform)]; !ok {
			complete = false
		}
	}
	if !complete {
		published, err := l.checksums(pin)
		if err != nil {
			return LockResult{}, err
		}
		for archive, sum := range published {
			if old, ok := next[archive]; ok && old != sum {
				return LockResult{}, fmt.Errorf("refusing: the release now gives %s the fingerprint %s, but %s records %s; it changed after it was recorded, so %s was left as it is", archive, sum, project.SumFile, old, project.SumFile)
			}
			next[archive] = sum
		}
	}

	// Only the pinned version is kept: git has the fingerprints of the versions pinned before.
	result := LockResult{Version: pin, Changed: !maps.Equal(recorded, next)}
	if !result.Changed {
		return result, nil
	}
	if err := writeAtomically(filepath.Join(l.Dir, project.SumFile), next.encode()); err != nil {
		return LockResult{}, fmt.Errorf("writing %s: %w", project.SumFile, err)
	}
	return result, nil
}

// checksums returns, from version's checksums.txt, the SHA-256 of each of its archives wf runs on.
// Every one of them must be listed exactly once; the release's other files are left out.
func (l *Launcher) checksums(version string) (sums, error) {
	data, err := l.get(l.ReleasesURL+"/v"+version+"/checksums.txt", maxChecksums)
	if errors.Is(err, errNotFound) {
		return nil, project.UserErrorf("%s pins wf %s, which has no release: %v", project.ConfigFile, version, err)
	}
	if err != nil {
		return nil, fmt.Errorf("downloading wf %s's checksums: %w", version, err)
	}
	wanted := map[string]bool{}
	for _, platform := range Platforms {
		wanted[Archive(version, platform)] = true
	}
	published := sums{}
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || !wanted[fields[1]] {
			continue
		}
		archive, sum := fields[1], fields[0]
		if len(sum) != sha256.Size*2 || strings.Trim(sum, "0123456789abcdef") != "" {
			return nil, fmt.Errorf("wf %s's checksums.txt gives %s no valid SHA-256", version, archive)
		}
		if _, ok := published[archive]; ok {
			return nil, fmt.Errorf("wf %s's checksums.txt lists %s twice", version, archive)
		}
		published[archive] = "sha256:" + sum
	}
	for _, archive := range slices.Sorted(maps.Keys(wanted)) {
		if _, ok := published[archive]; !ok {
			return nil, fmt.Errorf("wf %s's checksums.txt does not list %s, so nothing was recorded", version, archive)
		}
	}
	return published, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// writeAtomically writes data to file through a temporary file renamed into place, so a failure
// leaves the old file whole.
func writeAtomically(file string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(file), ".wf-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(temp.Name(), file)
}

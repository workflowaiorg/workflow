// Package launcher makes every checkout run the wf version it pins. The installed wf reads the pin,
// finds that release in a per-user cache or downloads it, checks the download against the
// project's .wf.sum, and says which binary to run in its place.
//
// What it reads before handing over, the wf: key of .wf.yml and the lines of .wf.sum, is read by
// every wf version from this one on, so neither shape ever changes.
package launcher

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/workflowaiorg/workflow/internal/project"
)

const (
	// ReleasesURL is where every release's archives and checksums.txt are downloaded from, under
	// v<version>/.
	ReleasesURL = "https://github.com/workflowaiorg/workflow/releases/download"
	// NoLaunchEnv set to 1 runs the installed wf whatever the project pins: for testing a build of
	// wf itself. It gives up the guarantee that every checkout runs the same version.
	NoLaunchEnv = "WF_NO_LAUNCH"
	// HandoverEnv names the version a launcher handed over to. The binary it started removes it,
	// so it reaches no wf that binary starts in turn. A release from before the launcher does not
	// know it and leaves it set, so a wf one of those starts refuses to run.
	HandoverEnv = "WF_HANDOVER_VERSION"
)

// Platforms are the os_arch pairs every release has an archive for.
var Platforms = []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"}

// The most a download may weigh: far above any real one, so a wrong URL cannot fill the memory.
const (
	maxArchive   = 200 << 20
	maxChecksums = 1 << 20
)

// versionPattern is what a pinned release version looks like. It ends up in a URL and in a cache
// path, so nothing else is accepted.
var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

// Launcher finds the binary of the wf version a project pins.
type Launcher struct {
	// Version is the version of the wf running.
	Version string
	// Dir is the project: the directory wf runs in.
	Dir string
	// CacheDir holds every downloaded version; empty means wf's folder in the user's cache.
	CacheDir string
	// ReleasesURL is where releases are downloaded from.
	ReleasesURL string
	// Platform is the os_arch pair of the binary to run.
	Platform string
	// NoLaunch runs this wf whatever the project pins.
	NoLaunch bool
	Client   *http.Client
}

// New returns the launcher of the wf running as version, in the project at dir.
func New(version, dir string) *Launcher {
	return &Launcher{
		Version:     version,
		Dir:         dir,
		ReleasesURL: ReleasesURL,
		Platform:    runtime.GOOS + "_" + runtime.GOARCH,
		NoLaunch:    os.Getenv(NoLaunchEnv) == "1",
		Client:      &http.Client{Timeout: 5 * time.Minute},
	}
}

// Target is what to run instead of the wf running: Binary, which is wf Version. An empty Binary
// means the wf running runs itself.
type Target struct {
	Binary  string
	Version string
}

// Resolve returns what runs the command args, downloading the pinned version when it is not cached
// yet. The wf running runs itself when told to, for wf lock, in a directory with no .wf.yml or no
// pin, and when it is the version pinned.
func (l *Launcher) Resolve(args []string) (Target, error) {
	// wf lock writes the .wf.sum a handover needs, so it is never handed over.
	if l.NoLaunch || (len(args) > 0 && args[0] == "lock") {
		return Target{}, nil
	}
	pin, _, err := readPin(l.Dir)
	if err != nil || pin == "" || pin == l.Version {
		return Target{}, err
	}
	if err := checkVersion(pin); err != nil {
		return Target{}, err
	}
	if !slices.Contains(Platforms, l.Platform) {
		return Target{}, fmt.Errorf("%s pins wf %s, but wf has no release for %s", project.ConfigFile, pin, l.Platform)
	}
	sums, err := readSum(l.Dir)
	if err != nil {
		return Target{}, err
	}
	archive := Archive(pin, l.Platform)
	want, ok := sums[archive]
	if !ok {
		return Target{}, project.UserErrorf("%s pins wf %s, but %s holds no fingerprint of %s; run wf lock and commit %s", project.ConfigFile, pin, project.SumFile, archive, project.SumFile)
	}

	cache, err := l.cacheDir()
	if err != nil {
		return Target{}, err
	}
	// Keyed by the fingerprint, so a binary one project checked is never run for another project
	// that recorded a different one for the same version.
	binary := filepath.Join(cache, pin, l.Platform, strings.TrimPrefix(want, "sha256:"), "wf")
	if info, err := os.Stat(binary); err == nil && info.Mode().IsRegular() {
		return Target{Binary: binary, Version: pin}, nil
	}
	if err := l.install(pin, archive, want, binary); err != nil {
		return Target{}, err
	}
	return Target{Binary: binary, Version: pin}, nil
}

// Environ returns environ for the binary of target, marked with the version it is handed over to.
func Environ(environ []string, target Target) []string {
	environ = slices.DeleteFunc(slices.Clone(environ), func(entry string) bool {
		return strings.HasPrefix(entry, HandoverEnv+"=")
	})
	return append(environ, HandoverEnv+"="+target.Version)
}

// Handover says whether a launcher started this wf, and removes the mark it left. A wf that is not
// the version it was started as fails, rather than hand over again.
func Handover(version string) (bool, error) {
	want, ok := os.LookupEnv(HandoverEnv)
	if !ok {
		return false, nil
	}
	if err := os.Unsetenv(HandoverEnv); err != nil {
		return true, fmt.Errorf("clearing %s: %w", HandoverEnv, err)
	}
	if want != version {
		return true, project.UserErrorf("was started as wf %s, but is wf %s; nothing was run. Unset %s", want, version, HandoverEnv)
	}
	return true, nil
}

// Archive returns the name of version's release archive for platform, an os_arch pair.
func Archive(version, platform string) string {
	return fmt.Sprintf("wf_%s_%s.tar.gz", version, platform)
}

// readPin returns the wf version the project at dir pins, reading only the wf: key, and whether it
// has a .wf.yml at all. No .wf.yml, or no wf: in it, pins nothing.
func readPin(dir string) (string, bool, error) {
	content, err := project.ReadText(filepath.Join(dir, project.ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", project.ConfigFile, err)
	}
	// Read as written: as a number, wf: 1.10 would be 1.1.
	var named struct {
		WF string `yaml:"wf"`
	}
	if err := yaml.Unmarshal(content, &named); err != nil {
		return "", true, project.UserErrorf("reading %s: %v", project.ConfigFile, err)
	}
	return named.WF, true, nil
}

func checkVersion(pin string) error {
	if !versionPattern.MatchString(pin) {
		return project.UserErrorf("%s: wf: %s is not a release version, such as 0.1.0", project.ConfigFile, pin)
	}
	return nil
}

func (l *Launcher) cacheDir() (string, error) {
	if l.CacheDir != "" {
		return l.CacheDir, nil
	}
	user, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding the cache folder for downloaded wf versions: %w", err)
	}
	return filepath.Join(user, "wf"), nil
}

// install downloads archive of version, checks it against want and puts its wf at binary. Nothing
// is left at binary unless the download matched.
func (l *Launcher) install(version, archive, want, binary string) error {
	data, err := l.get(l.ReleasesURL+"/v"+version+"/"+archive, maxArchive)
	if err != nil {
		return fmt.Errorf("downloading wf %s: %w", version, err)
	}
	if got := digest(data); got != want {
		return fmt.Errorf("refusing wf %s: the download of %s is %s, but %s records %s; nothing was run", version, archive, got, project.SumFile, want)
	}
	content, err := extract(data)
	if err != nil {
		return fmt.Errorf("refusing wf %s: %s %w; nothing was run", version, archive, err)
	}

	// The binary is written into a private directory beside its place, then the directory is
	// renamed into place whole, so a half-written binary is never found there.
	dir := filepath.Dir(binary)
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return fmt.Errorf("caching wf %s: %w", version, err)
	}
	staging, err := os.MkdirTemp(filepath.Dir(dir), ".staging-*")
	if err != nil {
		return fmt.Errorf("caching wf %s: %w", version, err)
	}
	defer os.RemoveAll(staging)
	// Flushed to disk before the rename: a crash must not leave a cut-off binary in the cache,
	// which is trusted from then on.
	if err := writeSynced(filepath.Join(staging, "wf"), content, 0o700); err != nil {
		return fmt.Errorf("caching wf %s: %w", version, err)
	}
	if err := os.Rename(staging, dir); err != nil {
		// Another wf may have cached the same checked binary meanwhile.
		if info, statErr := os.Stat(binary); statErr == nil && info.Mode().IsRegular() {
			return nil
		}
		return fmt.Errorf("caching wf %s: %w", version, err)
	}
	return nil
}

func writeSynced(file string, data []byte, mode os.FileMode) error {
	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// extract returns the wf binary in a release archive: its one regular file named wf.
func extract(archive []byte) ([]byte, error) {
	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("is not a gzip archive: %w", err)
	}
	files := tar.NewReader(compressed)
	var binary []byte
	found := false
	for {
		header, err := files.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cannot be read: %w", err)
		}
		if path.Clean(header.Name) != "wf" {
			continue
		}
		if found {
			return nil, errors.New("holds more than one wf")
		}
		if header.Typeflag != tar.TypeReg {
			return nil, errors.New("holds a wf that is not a plain file")
		}
		if binary, err = io.ReadAll(io.LimitReader(files, maxArchive)); err != nil {
			return nil, fmt.Errorf("cannot be read: %w", err)
		}
		found = true
	}
	if !found {
		return nil, errors.New("holds no wf")
	}
	return binary, nil
}

// errNotFound is a download the release does not have.
var errNotFound = errors.New("not found")

// get downloads url, refusing more than limit bytes.
func (l *Launcher) get(url string, limit int64) ([]byte, error) {
	response, err := l.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%s: %w", url, errNotFound)
	case response.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: %s", url, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: larger than any wf release file", url)
	}
	return data, nil
}

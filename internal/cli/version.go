package cli

import (
	"runtime/debug"
	"strings"
)

// develVersion is what a binary built without a release version or VCS information reports.
const develVersion = "devel"

// ResolveVersion returns the version a binary reports, without a leading "v": the one the release
// build injected, or else the module version go install recorded, or else develVersion.
func ResolveVersion(injected string) string {
	if injected != "" {
		return strings.TrimPrefix(injected, "v")
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return develVersion
	}
	return moduleVersion(info.Main.Version)
}

// moduleVersion turns the module version Go records into the one wf reports. A build without VCS
// information records "(devel)", or nothing at all.
func moduleVersion(recorded string) string {
	if recorded == "" || recorded == "(devel)" {
		return develVersion
	}
	return strings.TrimPrefix(recorded, "v")
}

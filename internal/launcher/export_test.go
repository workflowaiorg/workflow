package launcher

// Install exposes install to the external tests.
func (l *Launcher) Install(version, archive, want, binary string) error {
	return l.install(version, archive, want, binary)
}

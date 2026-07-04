package version

// Version and Commit are injected at build time via ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
)

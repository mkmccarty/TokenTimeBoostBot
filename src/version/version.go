package version

var (
	// Release is the current release or cycle of the application (e.g. "v2026.43")
	Release string
	// Version is the current version of the application (e.g. "v2026.43-3-gefa60477d")
	Version string
	// GitHash is the git hash of the commit that was used to build the application
	GitHash string
)

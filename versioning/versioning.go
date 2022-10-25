package versioning

// Embedded by --ldflags on build time
// Versioning should follow the SemVer guidelines
// https://semver.org/
var (
	// Version is the main version at the moment.
	Version   string // the main version at the moment
	Commit    string // the git commit that the binary was built on
	BuildTime string // the timestamp of the build
)

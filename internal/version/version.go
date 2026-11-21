// Package version holds the single source of truth for the application
// version. Bump Version and push to main; the version-tag workflow creates
// the matching git tag, which triggers the release build.
package version

// Version is the current application version (semver). A pre-release suffix
// (e.g. "-alpha.1") marks the build as a pre-release on GitHub.
const Version = "0.1.0-alpha.3"

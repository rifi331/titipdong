// Package version holds the build version, injected via -ldflags at build time.
package version

// Version is set by the build (e.g. "v0.4.0" or a git sha). Defaults to "dev".
var Version = "dev"

// Package buildinfo holds values set when the binary is built.
package buildinfo

// Version is the gclone version. The root main package sets it at start-up from its own
// version variable, which release builds override with -ldflags "-X main.version=...".
var Version = "0.1.1-dev"

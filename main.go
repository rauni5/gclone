// Command gclone clones a git repository and sets up everything it needs to run.
package main

import (
	"github.com/rauni5/gclone/internal/buildinfo"
	"github.com/rauni5/gclone/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=vX.Y.Z".
var version = "0.1.1-dev"

func main() {
	buildinfo.Version = version
	cli.Main()
}

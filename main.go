package main

import (
	"os"

	"github.com/varmakarthik12/owui-proxy/cmd"
)

// Build metadata — injected via ldflags at compile time.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	cmd.SetBuildInfo(Version, Commit, BuildDate)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

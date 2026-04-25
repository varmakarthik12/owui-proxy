package main

import (
	"os"

	"github.com/varmakarthik12/owui-proxy/cmd"
)

func main() {
	cmd.InitVersionInfo()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

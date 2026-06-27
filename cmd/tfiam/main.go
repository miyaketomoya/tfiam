package main

import (
	"os"

	"github.com/tfiam-dev/tfiam/internal/cli"
)

func main() {
	cmd := cli.NewRootCmd()
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		if e, ok := err.(cli.ExitError); ok {
			os.Exit(e.ExitCode())
		}
		os.Exit(1)
	}
}

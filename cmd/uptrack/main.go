package main

import (
	"os"

	"github.com/vaproh/uptrack/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		os.Exit(1)
	}
}

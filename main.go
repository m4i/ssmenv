package main

import (
	"os"
)

var version string

func main() {
	if err := (CLI{}).Run(os.Args); err != nil {
		os.Exit(1)
	}
}

package main

import (
	"os"

	"github.com/merdandt/LLM-wiki-dev/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

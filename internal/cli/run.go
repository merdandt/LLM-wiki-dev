package cli

import (
	"fmt"
	"io"
)

var Version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "llm-wiki %s\n", Version)
		return 0
	}
	fmt.Fprintln(stderr, "usage: llm-wiki <version|validate|status|init|finalize-init|migrate|hook|receipt|plugin>")
	return 2
}

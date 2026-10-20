package cli

import (
	"context"
	"fmt"
	"io"
)

const version = "dev"

const usage = "Usage: pare <command> [options]\n\nCommands:\n  scan    Scan a directory tree and write a JSON report\n"

func Run(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "pare %s\n", version)
		return 0
	}

	fmt.Fprint(stderr, usage)
	return 2
}

package cli

import (
	"fmt"
	"io"
	"os"
)

var usage = `relay is a goal-driven supervisor CLI.

Usage:
  relay <command> [arguments]

Commands:
  start    Start a new run from an objective spec
  status   Show current run status
  report   Print a run report
  kill     Stop a running task
  help     Show this help text
`

// Run executes the relay CLI and returns a process exit code.
func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return 1
	}

	switch args[0] {
	case "start":
		return printPlaceholder(stdout, "start")
	case "status":
		return printPlaceholder(stdout, "status")
	case "report":
		return printPlaceholder(stdout, "report")
	case "kill":
		return printPlaceholder(stdout, "kill")
	case "help", "-h", "--help":
		_, _ = io.WriteString(stdout, usage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 1
	}
}

func printPlaceholder(w io.Writer, command string) int {
	_, _ = fmt.Fprintf(w, "relay %s is not implemented yet\n", command)
	return 0
}

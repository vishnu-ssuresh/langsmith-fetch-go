package main

import (
	"fmt"
	"io"
)

func execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printRootUsage(stdout)
		return nil
	}

	switch args[0] {
	case "thread":
		return runThread(args[1:], stdout, stderr)
	case "traces":
		return runTraces(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printRootUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "langsmith-fetch-go")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  langsmith-fetch <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  thread    Fetch one thread")
	fmt.Fprintln(w, "  traces    List traces")
}

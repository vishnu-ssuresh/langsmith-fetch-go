package main

import (
	"fmt"
	"os"

	internalcmd "langsmith-fetch-go/internal/cmd"
)

func main() {
	if err := runWithArgs(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	return runWithArgs(os.Args[1:])
}

func runWithArgs(args []string) error {
	return internalcmd.Execute(args, os.Stdout, os.Stderr, internalcmd.NewDeps())
}

package main

import (
	"errors"
	"fmt"
	"os"

	"langsmith-fetch-go/internal/config"
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
	cfg := config.LoadFromEnv()
	if cfg.APIKey == "" {
		return errors.New("LANGSMITH_API_KEY (or LANGCHAIN_API_KEY) is required")
	}

	return execute(args, os.Stdout, os.Stderr)
}

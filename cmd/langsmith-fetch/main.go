package main

import (
	"errors"
	"fmt"
	"os"

	"langsmith-fetch-go/internal/app"
	"langsmith-fetch-go/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.LoadFromEnv()
	if cfg.APIKey == "" {
		return errors.New("LANGSMITH_API_KEY (or LANGCHAIN_API_KEY) is required")
	}

	if _, err := app.NewClientFromEnv(); err != nil {
		return fmt.Errorf("initialize langsmith client: %w", err)
	}

	fmt.Fprintln(os.Stdout, "langsmith-fetch-go initialized")
	return nil
}

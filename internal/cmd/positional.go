// positional.go provides shared parsing helpers for optional positional IDs.
package cmd

import (
	"fmt"
	"strings"
)

func popLeadingPositionalArg(args []string) (string, []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", args
	}
	return args[0], args[1:]
}

func resolveRequiredIDFromArgs(
	flagValue string,
	leadingPositional string,
	positionals []string,
	flagName string,
) (string, error) {
	id := strings.TrimSpace(flagValue)
	rest := positionals

	if id == "" {
		switch {
		case leadingPositional != "":
			id = leadingPositional
		case len(rest) > 0:
			id = rest[0]
			rest = rest[1:]
		}
	}

	if len(rest) > 0 {
		return "", fmt.Errorf("unexpected positional arguments: %v", rest)
	}
	if id == "" {
		return "", fmt.Errorf("--%s is required", flagName)
	}
	return id, nil
}

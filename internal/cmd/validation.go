// validation.go centralizes common CLI flag validation helpers.
package cmd

import "fmt"

func validateOutputFormat(value string) error {
	switch value {
	case "pretty", "json", "raw":
		return nil
	default:
		return fmt.Errorf("--format must be one of pretty|json|raw, got %q", value)
	}
}

func validatePositiveIntFlag(name string, value int) error {
	if value <= 0 {
		return fmt.Errorf("--%s must be > 0", name)
	}
	return nil
}

func validateMutuallyExclusiveStringFlags(
	leftName string,
	leftValue string,
	rightName string,
	rightValue string,
) error {
	if leftValue != "" && rightValue != "" {
		return fmt.Errorf("--%s and --%s are mutually exclusive", leftName, rightName)
	}
	return nil
}

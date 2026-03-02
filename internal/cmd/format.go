// format.go centralizes output format defaults and normalization.
package cmd

import "strings"

const outputFormatPretty = "pretty"

func configuredDefaultFormat(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return outputFormatPretty
	}
	return value
}

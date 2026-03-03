// statuserr centralizes HTTP status-to-error mapping for LangSmith accessors.
package statuserr

import (
	"fmt"
	"strings"

	langsmith "langsmith-sdk/go/langsmith"
)

// Wrap returns a contextual status error, including SDK typed sentinels
// for known auth/rate/transient status codes.
func Wrap(operation string, statusCode int, body []byte) error {
	typedErr := langsmith.ErrorForStatus(statusCode)
	bodyText := strings.TrimSpace(string(body))

	if typedErr != nil {
		if bodyText == "" {
			return fmt.Errorf("%s returned status %d: %w", operation, statusCode, typedErr)
		}
		return fmt.Errorf("%s returned status %d: %w: %s", operation, statusCode, typedErr, bodyText)
	}

	if bodyText == "" {
		return fmt.Errorf("%s returned status %d", operation, statusCode)
	}
	return fmt.Errorf("%s returned status %d: %s", operation, statusCode, bodyText)
}

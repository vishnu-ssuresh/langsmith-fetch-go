package statuserr

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	langsmith "langsmith-sdk/go/langsmith"
)

func TestWrap_MapsTypedSDKErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   error
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: langsmith.ErrUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, want: langsmith.ErrForbidden},
		{name: "not found", status: http.StatusNotFound, want: langsmith.ErrNotFound},
		{name: "rate limited", status: http.StatusTooManyRequests, want: langsmith.ErrRateLimited},
		{name: "transient", status: http.StatusInternalServerError, want: langsmith.ErrTransient},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Wrap("op", tt.status, []byte("mock body"))
			if err == nil {
				t.Fatal("Wrap() error = nil, want non-nil")
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("Wrap() error = %v, want errors.Is(_, %v)", err, tt.want)
			}
		})
	}
}

func TestWrap_NonTypedStatusPreservesContext(t *testing.T) {
	t.Parallel()

	err := Wrap("feedback: list feedback", http.StatusBadRequest, []byte("bad request"))
	if err == nil {
		t.Fatal("Wrap() error = nil, want non-nil")
	}
	if errors.Is(err, langsmith.ErrUnauthorized) ||
		errors.Is(err, langsmith.ErrForbidden) ||
		errors.Is(err, langsmith.ErrNotFound) ||
		errors.Is(err, langsmith.ErrRateLimited) ||
		errors.Is(err, langsmith.ErrTransient) {
		t.Fatalf("Wrap() error = %v, got unexpected typed SDK error", err)
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("Wrap() error = %v, want status code in message", err)
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("Wrap() error = %v, want response body in message", err)
	}
}

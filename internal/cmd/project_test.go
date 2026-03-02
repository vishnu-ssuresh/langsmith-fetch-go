// project_test.go verifies project UUID resolution precedence behavior.
package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/config"
)

type fakeProjectResolver struct {
	name string
	id   string
	err  error
}

func (f *fakeProjectResolver) ResolveProjectUUID(_ context.Context, name string) (string, error) {
	f.name = name
	if f.err != nil {
		return "", f.err
	}
	return f.id, nil
}

func TestResolveProjectID_PrefersExplicitFlag(t *testing.T) {
	t.Parallel()

	projectID, err := resolveProjectID("explicit-id", config.Values{ProjectUUID: "env-id"}, Deps{})
	if err != nil {
		t.Fatalf("resolveProjectID() error = %v", err)
	}
	if projectID != "explicit-id" {
		t.Fatalf("projectID = %q, want %q", projectID, "explicit-id")
	}
}

func TestResolveProjectID_UsesConfigProjectUUID(t *testing.T) {
	t.Parallel()

	projectID, err := resolveProjectID("", config.Values{ProjectUUID: "env-id"}, Deps{})
	if err != nil {
		t.Fatalf("resolveProjectID() error = %v", err)
	}
	if projectID != "env-id" {
		t.Fatalf("projectID = %q, want %q", projectID, "env-id")
	}
}

func TestResolveProjectID_UsesProjectNameLookup(t *testing.T) {
	t.Parallel()

	fake := &fakeProjectResolver{id: "resolved-id"}
	projectID, err := resolveProjectID("", config.Values{ProjectName: "my-project"}, Deps{
		NewProjectResolver: func(config.Values) (projectResolver, error) { return fake, nil },
	})
	if err != nil {
		t.Fatalf("resolveProjectID() error = %v", err)
	}
	if projectID != "resolved-id" {
		t.Fatalf("projectID = %q, want %q", projectID, "resolved-id")
	}
	if fake.name != "my-project" {
		t.Fatalf("resolver name = %q, want %q", fake.name, "my-project")
	}
}

func TestResolveProjectID_ProjectResolverInitError(t *testing.T) {
	t.Parallel()

	_, err := resolveProjectID("", config.Values{ProjectName: "my-project"}, Deps{
		NewProjectResolver: func(config.Values) (projectResolver, error) {
			return nil, errors.New("boom")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "initialize project resolver") {
		t.Fatalf("resolveProjectID() error = %v, want init resolver error", err)
	}
}

func TestResolveProjectID_ProjectLookupError(t *testing.T) {
	t.Parallel()

	fake := &fakeProjectResolver{err: errors.New("lookup failed")}
	_, err := resolveProjectID("", config.Values{ProjectName: "my-project"}, Deps{
		NewProjectResolver: func(config.Values) (projectResolver, error) { return fake, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "resolve project") {
		t.Fatalf("resolveProjectID() error = %v, want resolve project error", err)
	}
}

func TestResolveProjectID_MissingProject(t *testing.T) {
	t.Parallel()

	_, err := resolveProjectID("", config.Values{}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("resolveProjectID() error = %v, want missing project error", err)
	}
}

// project.go resolves project UUID from flags, env-config, or project-name lookup.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"langsmith-fetch-go/internal/config"
)

type projectResolver interface {
	ResolveProjectUUID(context.Context, string) (string, error)
}

func resolveProjectID(input string, cfg config.Values, deps Deps) (string, error) {
	if projectID := strings.TrimSpace(input); projectID != "" {
		return projectID, nil
	}
	if cfg.ProjectUUID != "" {
		return cfg.ProjectUUID, nil
	}
	if cfg.ProjectName != "" {
		resolver, err := deps.NewProjectResolver(cfg)
		if err != nil {
			return "", fmt.Errorf("initialize project resolver: %w", err)
		}
		projectID, err := resolver.ResolveProjectUUID(context.Background(), cfg.ProjectName)
		if err != nil {
			return "", fmt.Errorf("resolve project %q: %w", cfg.ProjectName, err)
		}
		if deps.CacheProjectUUID != nil {
			_ = deps.CacheProjectUUID(cfg.ProjectName, projectID)
		}
		return projectID, nil
	}
	return "", errors.New("--project-id is required (or set LANGSMITH_PROJECT_UUID or LANGSMITH_PROJECT)")
}

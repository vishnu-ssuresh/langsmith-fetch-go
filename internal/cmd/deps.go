// deps.go wires production dependencies for command handlers.
package cmd

import (
	"strings"

	"github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"

	"langsmith-fetch-go/internal/config"
	coresingle "langsmith-fetch-go/internal/core/single"
	corethreads "langsmith-fetch-go/internal/core/threads"
	"langsmith-fetch-go/internal/core/traces"
	langsmithfeedback "langsmith-fetch-go/internal/langsmith/feedback"
	langsmithprojects "langsmith-fetch-go/internal/langsmith/projects"
	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

// Deps contains root command dependencies.
type Deps struct {
	LoadConfig          func() config.Values
	NewTraceGetter      func(config.Values) (traceGetter, error)
	NewFeedbackAccessor func(config.Values) (traceFeedbackAccessor, error)
	NewTracesLister     func(config.Values) (tracesLister, error)
	NewThreadGetter     func(config.Values) (threadGetter, error)
	NewThreadsLister    func(config.Values) (threadsLister, error)
	NewProjectResolver  func(config.Values) (projectResolver, error)
	CacheProjectUUID    func(projectName string, projectUUID string) error
}

// NewDeps returns production command dependencies.
func NewDeps() Deps {
	return Deps{
		LoadConfig: config.Load,
		NewTraceGetter: func(cfg config.Values) (traceGetter, error) {
			client := newSDKClient(cfg)
			runsAccessor, err := langsmithruns.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return coresingle.NewTraceService(runsAccessor)
		},
		NewFeedbackAccessor: func(cfg config.Values) (traceFeedbackAccessor, error) {
			client := newSDKClient(cfg)
			return langsmithfeedback.NewAccessor(client)
		},
		NewTracesLister: func(cfg config.Values) (tracesLister, error) {
			client := newSDKClient(cfg)
			runsAccessor, err := langsmithruns.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			feedbackAccessor, err := langsmithfeedback.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return traces.New(runsAccessor, feedbackAccessor)
		},
		NewThreadGetter: func(cfg config.Values) (threadGetter, error) {
			client := newSDKClient(cfg)
			threadsAccessor, err := langsmiththreads.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return coresingle.NewThreadService(threadsAccessor)
		},
		NewThreadsLister: func(cfg config.Values) (threadsLister, error) {
			client := newSDKClient(cfg)
			runsAccessor, err := langsmithruns.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			threadsAccessor, err := langsmiththreads.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return corethreads.NewLister(runsAccessor, threadsAccessor)
		},
		NewProjectResolver: func(cfg config.Values) (projectResolver, error) {
			client := newSDKClient(cfg)
			return langsmithprojects.NewAccessor(client)
		},
		CacheProjectUUID: cacheProjectUUIDToConfigFile,
	}
}

func (d Deps) withDefaults() Deps {
	if d.LoadConfig == nil {
		d.LoadConfig = config.Load
	}
	if d.NewTracesLister == nil {
		d.NewTracesLister = NewDeps().NewTracesLister
	}
	if d.NewFeedbackAccessor == nil {
		d.NewFeedbackAccessor = NewDeps().NewFeedbackAccessor
	}
	if d.NewTraceGetter == nil {
		d.NewTraceGetter = NewDeps().NewTraceGetter
	}
	if d.NewThreadGetter == nil {
		d.NewThreadGetter = NewDeps().NewThreadGetter
	}
	if d.NewThreadsLister == nil {
		d.NewThreadsLister = NewDeps().NewThreadsLister
	}
	if d.NewProjectResolver == nil {
		d.NewProjectResolver = NewDeps().NewProjectResolver
	}
	if d.CacheProjectUUID == nil {
		d.CacheProjectUUID = NewDeps().CacheProjectUUID
	}
	return d
}

func newSDKClient(cfg config.Values) *langsmith.Client {
	opts := []option.RequestOption{}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.Endpoint != "" {
		opts = append(opts, option.WithBaseURL(cfg.Endpoint))
	}
	if cfg.WorkspaceID != "" {
		opts = append(opts, option.WithTenantID(cfg.WorkspaceID))
	}
	return langsmith.NewClient(opts...)
}

func cacheProjectUUIDToConfigFile(projectName string, projectUUID string) error {
	projectName = strings.TrimSpace(projectName)
	projectUUID = strings.TrimSpace(projectUUID)
	if projectName == "" || projectUUID == "" {
		return nil
	}

	values, err := config.LoadFromFile("")
	if err != nil {
		return err
	}
	if values.ProjectName == projectName && values.ProjectUUID == projectUUID {
		return nil
	}
	values.ProjectName = projectName
	values.ProjectUUID = projectUUID
	return config.SaveToFile("", values)
}

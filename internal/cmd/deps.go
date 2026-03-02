// deps.go wires production dependencies for command handlers.
package cmd

import (
	langsmith "langsmith-sdk/go/langsmith"

	"langsmith-fetch-go/internal/config"
	coresingle "langsmith-fetch-go/internal/core/single"
	corethreads "langsmith-fetch-go/internal/core/threads"
	"langsmith-fetch-go/internal/core/traces"
	langsmithprojects "langsmith-fetch-go/internal/langsmith/projects"
	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

// Deps contains root command dependencies.
//
// Keeping these as function fields makes command code easy to unit test
// without real process environment access.
type Deps struct {
	LoadConfig         func() config.Values
	NewTraceGetter     func(config.Values) (traceGetter, error)
	NewTracesLister    func(config.Values) (tracesLister, error)
	NewThreadGetter    func(config.Values) (threadGetter, error)
	NewThreadsLister   func(config.Values) (threadsLister, error)
	NewProjectResolver func(config.Values) (projectResolver, error)
}

// NewDeps returns production command dependencies.
func NewDeps() Deps {
	return Deps{
		LoadConfig: config.LoadFromEnv,
		NewTraceGetter: func(cfg config.Values) (traceGetter, error) {
			client, err := newSDKClient(cfg)
			if err != nil {
				return nil, err
			}
			runsAccessor, err := langsmithruns.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return coresingle.NewTraceService(runsAccessor)
		},
		NewTracesLister: func(cfg config.Values) (tracesLister, error) {
			client, err := newSDKClient(cfg)
			if err != nil {
				return nil, err
			}
			runsAccessor, err := langsmithruns.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return traces.New(runsAccessor)
		},
		NewThreadGetter: func(cfg config.Values) (threadGetter, error) {
			client, err := newSDKClient(cfg)
			if err != nil {
				return nil, err
			}
			threadsAccessor, err := langsmiththreads.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return corethreads.New(threadsAccessor)
		},
		NewThreadsLister: func(cfg config.Values) (threadsLister, error) {
			client, err := newSDKClient(cfg)
			if err != nil {
				return nil, err
			}
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
			client, err := newSDKClient(cfg)
			if err != nil {
				return nil, err
			}
			return langsmithprojects.NewAccessor(client)
		},
	}
}

func (d Deps) withDefaults() Deps {
	if d.LoadConfig == nil {
		d.LoadConfig = config.LoadFromEnv
	}
	if d.NewTracesLister == nil {
		d.NewTracesLister = NewDeps().NewTracesLister
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
	return d
}

func newSDKClient(cfg config.Values) (*langsmith.Client, error) {
	return langsmith.NewClient(langsmith.ClientOptions{
		APIKey:      cfg.APIKey,
		WorkspaceID: cfg.WorkspaceID,
		Endpoint:    cfg.Endpoint,
	})
}

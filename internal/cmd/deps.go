// deps.go wires production dependencies for command handlers.
package cmd

import (
	langsmith "langsmith-sdk/go/langsmith"

	"langsmith-fetch-go/internal/config"
	corethreads "langsmith-fetch-go/internal/core/threads"
	"langsmith-fetch-go/internal/core/traces"
	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

// Deps contains root command dependencies.
//
// Keeping these as function fields makes command code easy to unit test
// without real process environment access.
type Deps struct {
	LoadConfig      func() config.Values
	NewTracesLister func(config.Values) (tracesLister, error)
	NewThreadGetter func(config.Values) (threadGetter, error)
}

// NewDeps returns production command dependencies.
func NewDeps() Deps {
	return Deps{
		LoadConfig: config.LoadFromEnv,
		NewTracesLister: func(cfg config.Values) (tracesLister, error) {
			client, err := langsmith.NewClient(langsmith.ClientOptions{
				APIKey:      cfg.APIKey,
				WorkspaceID: cfg.WorkspaceID,
				Endpoint:    cfg.Endpoint,
			})
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
			client, err := langsmith.NewClient(langsmith.ClientOptions{
				APIKey:      cfg.APIKey,
				WorkspaceID: cfg.WorkspaceID,
				Endpoint:    cfg.Endpoint,
			})
			if err != nil {
				return nil, err
			}
			threadsAccessor, err := langsmiththreads.NewAccessor(client)
			if err != nil {
				return nil, err
			}
			return corethreads.New(threadsAccessor)
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
	if d.NewThreadGetter == nil {
		d.NewThreadGetter = NewDeps().NewThreadGetter
	}
	return d
}

package cmd

import "langsmith-fetch-go/internal/config"

// Deps contains root command dependencies.
//
// Keeping these as function fields makes command code easy to unit test
// without real process environment access.
type Deps struct {
	LoadConfig func() config.Values
}

// NewDeps returns production command dependencies.
func NewDeps() Deps {
	return Deps{
		LoadConfig: config.LoadFromEnv,
	}
}

func (d Deps) withDefaults() Deps {
	if d.LoadConfig == nil {
		d.LoadConfig = config.LoadFromEnv
	}
	return d
}

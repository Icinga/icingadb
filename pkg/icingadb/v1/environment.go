package v1

import (
	"context"
	"github.com/icinga/icingadb/pkg/types"
)

type Environment struct {
	EntityWithoutChecksum `json:",inline"`
	Name                  types.String `json:"name"`
}

// NewContext returns a new Context that carries this Environment as value.
func (e *Environment) NewContext(parent context.Context) context.Context {
	return context.WithValue(parent, environmentContextKey, e)
}

// Meta returns the EnvironmentMeta for this Environment.
func (e *Environment) Meta() *EnvironmentMeta {
	return &EnvironmentMeta{EnvironmentId: e.Id}
}

// EnvironmentFromContext returns the Environment value stored in ctx, if any:
//
// 	e, ok := EnvironmentFromContext(ctx)
// 	if !ok {
// 		// Error handling.
// 	}
func EnvironmentFromContext(ctx context.Context) (*Environment, bool) {
	if e, ok := ctx.Value(environmentContextKey).(*Environment); ok {
		return e, true
	}

	return nil, false
}

// environmentContextKey is the key for Environment values in contexts.
// It's not exported, so callers use Environment.NewContext and EnvironmentFromContext
// instead of using that key directly.
var environmentContextKey contextKey

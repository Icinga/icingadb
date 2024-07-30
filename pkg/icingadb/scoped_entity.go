package icingadb

import (
	"context"
	"github.com/icinga/icinga-go-library/database"
)

// ScopedEntity combines an entity and a scope that specifies
// the WHERE conditions that entities of the
// enclosed entity type must satisfy in order to be SELECTed.
type ScopedEntity struct {
	database.Entity
	scope interface{}
}

// Scope implements the contracts.Scoper interface.
func (e ScopedEntity) Scope() interface{} {
	return e.scope
}

// TableName implements the contracts.TableNamer interface.
func (e ScopedEntity) TableName() string {
	return database.TableName(e.Entity)
}

// NewScopedEntity returns a new ScopedEntity.
func NewScopedEntity(entity database.Entity, scope interface{}) *ScopedEntity {
	return &ScopedEntity{
		Entity: entity,
		scope:  scope,
	}
}

// OnSuccessApplyAndSendTo applies the provided callback to all the rows of type "T" and streams them to the
// passed channel of type "U". The resulting closure is called with a context and stops as soon as this context
// is canceled or when there are no more records to stream.
// TODO(yh): Move this to our Icinga GO Library!
func OnSuccessApplyAndSendTo[T any, U any](ch chan<- U, f func(T) U) database.OnSuccess[T] {
	return func(ctx context.Context, rows []T) error {
		for _, row := range rows {
			select {
			case ch <- f(row):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}
}

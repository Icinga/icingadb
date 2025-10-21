package icingadb

import (
	"context"
	"github.com/icinga/icinga-go-library/database"
)

// EntitiesById is a map of key-contracts.Entity pairs.
type EntitiesById map[string]database.Entity

// Keys returns the keys.
func (ebi EntitiesById) Keys() []string {
	keys := make([]string, 0, len(ebi))
	for k := range ebi {
		keys = append(keys, k)
	}

	return keys
}

// IDs returns the contracts.ID of the entities.
func (ebi EntitiesById) IDs() []any {
	ids := make([]any, 0, len(ebi))
	for _, v := range ebi {
		ids = append(ids, v.ID())
	}

	return ids
}

// Entities streams the entities on a returned channel.
func (ebi EntitiesById) Entities(ctx context.Context) <-chan database.Entity {
	entities := make(chan database.Entity)

	go func() {
		defer close(entities)

		for _, v := range ebi {
			select {
			case <-ctx.Done():
				return
			case entities <- v:
			}
		}
	}()

	return entities
}

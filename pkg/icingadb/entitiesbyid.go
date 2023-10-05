package icingadb

import (
	"context"
	"github.com/icinga/icingadb/pkg/database"
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
func (ebi EntitiesById) IDs() []interface{} {
	ids := make([]interface{}, 0, len(ebi))
	for _, v := range ebi {
		ids = append(ids, v.(database.IDer).ID())
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

package icingadb

import (
	"context"
	"github.com/icinga/icingadb/pkg/contracts"
)

// EntitiesById is a map of key-contracts.Entity pairs.
type EntitiesById map[string]contracts.Entity

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
		ids = append(ids, v.(contracts.IDer).ID())
	}

	return ids
}

// Entities streams the entities on a returned channel.
func (ebi EntitiesById) Entities(ctx context.Context) <-chan contracts.Entity {
	entities := make(chan contracts.Entity)

	go func() {
		defer close(entities)

		for k, v := range ebi {
			delete(ebi, k)

			select {
			case <-ctx.Done():
				return
			case entities <- v:
			}
		}
	}()

	return entities
}

package icingadb

import (
	"context"
	"encoding/hex"
	"github.com/icinga/icingadb/pkg/contracts"
)

// EntitiesById is a map contracts.ID-contracts.Entity pairs.
type EntitiesById map[contracts.ID]contracts.Entity

// Keys returns the keys.
func (ebi EntitiesById) Keys() []string {
	keys := make([]string, 0, len(ebi))
	for k := range ebi {
		keys = append(keys, hex.EncodeToString(k[:]))
	}

	return keys
}

// IDs returns the contracts.ID of the entities.
func (ebi EntitiesById) IDs() []interface{} {
	ids := make([]interface{}, 0, len(ebi))
	for k := range ebi {
		ids = append(ids, k)
	}

	return ids
}

// Entities streams the entities on a returned channel.
func (ebi EntitiesById) Entities(ctx context.Context) <-chan contracts.Entity {
	entities := make(chan contracts.Entity)

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

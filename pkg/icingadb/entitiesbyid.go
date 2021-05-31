package icingadb

import (
	"context"
	"github.com/icinga/icingadb/pkg/contracts"
)

type EntitiesById map[string]contracts.Entity

func (ebi EntitiesById) Keys() []string {
	keys := make([]string, 0, len(ebi))
	for k := range ebi {
		keys = append(keys, k)
	}

	return keys
}

func (ebi EntitiesById) IDs() []interface{} {
	ids := make([]interface{}, 0, len(ebi))
	for _, v := range ebi {
		ids = append(ids, v.(contracts.IDer).ID())
	}

	return ids
}

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

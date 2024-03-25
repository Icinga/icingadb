package history

import (
	"github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/icinga/icingadb/pkg/structify"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/redis/go-redis/v9"
	"reflect"
)

var slaStateStructify = structify.MakeMapStructifier(reflect.TypeOf((*history.SlaHistoryState)(nil)).Elem(), "json")

func stateHistoryToSlaEntity(entry redis.XMessage) ([]history.UpserterEntity, error) {
	slaStateInterface, err := slaStateStructify(entry.Values)
	if err != nil {
		return nil, err
	}
	slaState := slaStateInterface.(*history.SlaHistoryState)

	if slaState.StateType != types.StateHard {
		// only hard state changes are relevant for SLA history, discard all others
		return nil, nil
	}

	return []history.UpserterEntity{slaState}, nil
}

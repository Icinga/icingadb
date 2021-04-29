package v1

import (
	"encoding/json"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/pkg/errors"
)

// StatsMessage represents a message from the Redis stream icinga:stats.
type StatsMessage map[string]interface{}

func (m StatsMessage) Raw() map[string]interface{} {
	return m
}

func (m StatsMessage) IcingaStatus() (*IcingaStatus, error) {
	if s, ok := m["IcingaApplication"].(string); ok {
		var envelope struct {
			Status struct {
				IcingaApplication struct {
					IcingaStatus `json:"app"`
				} `json:"icingaapplication"`
			} `json:"status"`
		}

		if err := json.Unmarshal([]byte(s), &envelope); err != nil {
			return nil, errors.Wrap(err, "can't parse Icinga 2 status")
		}

		return &envelope.Status.IcingaApplication.IcingaStatus, nil
	}

	return nil, errors.New("bad message")
}

func (m StatsMessage) Time() (*types.UnixMilli, error) {
	if s, ok := m["timestamp"].(string); ok {
		var t types.UnixMilli

		if err := json.Unmarshal([]byte(s), &t); err != nil {
			return nil, errors.Wrap(err, "can't parse timestamp")
		}

		return &t, nil
	}

	return nil, errors.New("bad message")
}

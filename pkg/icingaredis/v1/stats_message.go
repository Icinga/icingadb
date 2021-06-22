package v1

import (
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/pkg/errors"
)

// StatsMessage represents a message from the Redis stream icinga:stats.
type StatsMessage map[string]interface{}

// Raw returns the key-value pairs of the message.
func (m StatsMessage) Raw() map[string]interface{} {
	return m
}

// IcingaStatus extracts Icinga status information from the message into IcingaStatus and returns it.
func (m StatsMessage) IcingaStatus() (*IcingaStatus, error) {
	if s, ok := m["IcingaApplication"].(string); ok {
		var envelope struct {
			Status struct {
				IcingaApplication struct {
					IcingaStatus `json:"app"`
				} `json:"icingaapplication"`
			} `json:"status"`
		}

		if err := internal.UnmarshalJSON([]byte(s), &envelope); err != nil {
			return nil, err
		}

		return &envelope.Status.IcingaApplication.IcingaStatus, nil
	}

	return nil, errors.Errorf(`bad message %#v. "IcingaApplication" missing`, m)
}

// Time extracts the timestamp of the message into types.UnixMilli and returns it.
func (m StatsMessage) Time() (*types.UnixMilli, error) {
	if s, ok := m["timestamp"].(string); ok {
		var t types.UnixMilli

		if err := internal.UnmarshalJSON([]byte(s), &t); err != nil {
			return nil, err
		}

		return &t, nil
	}

	return nil, errors.Errorf(`bad message %#v. "timestamp" missing`, m)
}

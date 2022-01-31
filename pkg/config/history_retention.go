package config

import (
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/pkg/errors"
	"time"
)

// HistoryRetention defines configuration for history retention.
type HistoryRetention struct {
	Days     uint64                   `yaml:"days"`
	Interval time.Duration            `yaml:"interval" default:"1h"`
	Count    uint64                   `yaml:"count" default:"5000"`
	Options  history.RetentionOptions `yaml:"options"`
}

// Validate checks constraints in the supplied retention configuration and
// returns an error if they are violated.
func (r *HistoryRetention) Validate() error {
	if r.Interval <= 0 {
		return errors.New("retention interval must be positive")
	}

	return r.Options.Validate()
}

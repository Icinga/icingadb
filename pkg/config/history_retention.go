package config

import (
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/pkg/errors"
	"time"
)

// Retention defines configuration for history retention.
type Retention struct {
	HistoryDays uint64                   `yaml:"history-days"`
	SlaDays     uint64                   `yaml:"sla-days"`
	Interval    time.Duration            `yaml:"interval" default:"1h"`
	Count       uint64                   `yaml:"count" default:"5000"`
	Options     history.RetentionOptions `yaml:"options"`
}

// Validate checks constraints in the supplied retention configuration and
// returns an error if they are violated.
func (r *Retention) Validate() error {
	if r.Interval <= 0 {
		return errors.New("retention interval must be positive")
	}

	if r.Count == 0 {
		return errors.New("count must be greater than zero")
	}

	return r.Options.Validate()
}

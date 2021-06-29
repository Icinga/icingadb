package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/pkg/cleanup"
	"github.com/icinga/icingadb/pkg/icingadb"
	"go.uber.org/zap"
)

// Cleanup configuration.
type Cleanup struct {
	HistoryTables   cleanup.Tables `yaml:"history,omitempty"`
	cleanup.Options `yaml:"options"`
}

// NewCleanup prepares Cleanup configuration,
// calls cleanup.NewClient, but returns *cleanup.Cleanup.
func (cu *Cleanup) NewCleanup(db *icingadb.DB, logger *zap.SugaredLogger) *cleanup.Cleanup {
	return cleanup.NewCleanup(db, cu.HistoryTables, &cu.Options, logger)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (cu *Cleanup) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(cu); err != nil {
		return err
	}
	// Prevent recursion.
	type self Cleanup
	if err := unmarshal(&(*self)(cu).HistoryTables); err != nil {
		return err
	}

	return nil
}

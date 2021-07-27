package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/pkg/cleanup"
)

// Cleanup configuration.
type Cleanup struct {
	cleanup.HistoryRetention `yaml:"history"`
	cleanup.Options          `yaml:"options"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (cu *Cleanup) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := defaults.Set(cu); err != nil {
		return err
	}
	// Prevent recursion.
	type self Cleanup
	if err := unmarshal((*self)(cu)); err != nil {
		return err
	}

	return nil
}

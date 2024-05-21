package v1

import (
	"github.com/icinga/icingadb/pkg/database"
)

type ActionUrl struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	ActionUrl             string `json:"action_url"`
}

type NotesUrl struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NotesUrl              string `json:"notes_url"`
}

type IconImage struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	IconImage             string `json:"icon_image"`
}

func NewActionUrl() database.Entity {
	return &ActionUrl{}
}

func NewNotesUrl() database.Entity {
	return &NotesUrl{}
}

func NewIconImage() database.Entity {
	return &IconImage{}
}

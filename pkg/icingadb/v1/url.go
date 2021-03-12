package v1

import "github.com/icinga/icingadb/pkg/contracts"

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

func NewActionUrl() contracts.Entity {
	return &ActionUrl{}
}

func NewNotesUrl() contracts.Entity {
	return &NotesUrl{}
}

func NewIconImage() contracts.Entity {
	return &IconImage{}
}

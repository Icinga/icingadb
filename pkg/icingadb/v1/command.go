package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Command struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameCiMeta         `json:",inline"`
	ZoneId             types.Binary `json:"zone_id"`
	Command            string       `json:"command"`
	Timeout            uint32       `json:"timeout"`
}

type CommandArgument struct {
	EntityWithChecksum  `json:",inline"`
	EnvironmentMeta     `json:",inline"`
	CommandId           types.Binary `json:"command_id"`
	ArgumentKey         string       `json:"argument_key"`
	ArgumentValue       types.String `json:"value"`
	ArgumentOrder       types.Int    `json:"order"`
	Description         types.String `json:"description"`
	ArgumentKeyOverride types.String `json:"key"`
	RepeatKey           types.Bool   `json:"repeat_key"`
	Required            types.Bool   `json:"required"`
	SetIf               types.String `json:"set_if"`
	SkipKey             types.Bool   `json:"skip_key"`
}

type CommandEnvvar struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	CommandId          types.Binary `json:"command_id"`
	EnvvarKey          string       `json:"envvar_key"`
	EnvvarValue        string       `json:"value"`
}

type CommandCustomvar struct {
	CustomvarMeta `json:",inline"`
	CommandId     types.Binary `json:"object_id"`
}

type Checkcommand struct {
	Command `json:",inline"`
}

type CheckcommandArgument struct {
	CommandArgument `json:",inline"`
}

type CheckcommandEnvvar struct {
	CommandEnvvar `json:",inline"`
}

type CheckcommandCustomvar struct {
	CommandCustomvar `json:",inline"`
}

type Eventcommand struct {
	Command `json:",inline"`
}

type EventcommandArgument struct {
	CommandArgument `json:",inline"`
}

type EventcommandEnvvar struct {
	CommandEnvvar `json:",inline"`
}

type EventcommandCustomvar struct {
	CommandCustomvar `json:",inline"`
}

type Notificationcommand struct {
	Command `json:",inline"`
}

type NotificationcommandArgument struct {
	CommandArgument `json:",inline"`
}

type NotificationcommandEnvvar struct {
	CommandEnvvar `json:",inline"`
}

type NotificationcommandCustomvar struct {
	CommandCustomvar `json:",inline"`
}

func NewCheckcommand() contracts.Entity {
	return &Checkcommand{}
}

func NewCheckcommandArgument() contracts.Entity {
	return &CheckcommandArgument{}
}

func NewCheckcommandEnvvar() contracts.Entity {
	return &CheckcommandEnvvar{}
}

func NewCheckcommandCustomvar() contracts.Entity {
	return &CheckcommandCustomvar{}
}

func NewEventcommand() contracts.Entity {
	return &Eventcommand{}
}

func NewEventcommandArgument() contracts.Entity {
	return &EventcommandArgument{}
}

func NewEventcommandEnvvar() contracts.Entity {
	return &EventcommandEnvvar{}
}

func NewEventcommandCustomvar() contracts.Entity {
	return &EventcommandCustomvar{}
}

func NewNotificationcommand() contracts.Entity {
	return &Notificationcommand{}
}

func NewNotificationcommandArgument() contracts.Entity {
	return &NotificationcommandArgument{}
}

func NewNotificationcommandEnvvar() contracts.Entity {
	return &NotificationcommandEnvvar{}
}

func NewNotificationcommandCustomvar() contracts.Entity {
	return &NotificationcommandCustomvar{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Command)(nil)
	_ contracts.Initer = (*Checkcommand)(nil)
	_ contracts.Initer = (*Eventcommand)(nil)
	_ contracts.Initer = (*Notificationcommand)(nil)
)

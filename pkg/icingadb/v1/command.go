package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/contracts"
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
	ArgumentKey         string       `json:"argument_key"`
	ArgumentValue       types.String `json:"value"`
	ArgumentOrder       types.Int    `json:"order"`
	Description         types.String `json:"description"`
	ArgumentKeyOverride types.String `json:"key"`
	RepeatKey           types.Bool   `json:"repeat_key"`
	Required            types.Bool   `json:"required"`
	SetIf               types.String `json:"set_if"`
	Separator           types.String `json:"separator"`
	SkipKey             types.Bool   `json:"skip_key"`
}

// Init implements the contracts.Initer interface.
func (ca *CommandArgument) Init() {
	ca.RepeatKey = types.Bool{
		Bool:  true,
		Valid: true,
	}

	ca.Required = types.Bool{
		Bool:  false,
		Valid: true,
	}

	ca.SkipKey = types.Bool{
		Bool:  false,
		Valid: true,
	}
}

type CommandEnvvar struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	EnvvarKey          string `json:"envvar_key"`
	EnvvarValue        string `json:"value"`
}

type Checkcommand struct {
	Command `json:",inline"`
}

type CheckcommandArgument struct {
	CommandArgument `json:",inline"`
	CheckcommandId  types.Binary `json:"checkcommand_id"`
}

type CheckcommandEnvvar struct {
	CommandEnvvar  `json:",inline"`
	CheckcommandId types.Binary `json:"checkcommand_id"`
}

type CheckcommandCustomvar struct {
	CustomvarMeta  `json:",inline"`
	CheckcommandId types.Binary `json:"checkcommand_id"`
}

type Eventcommand struct {
	Command `json:",inline"`
}

type EventcommandArgument struct {
	CommandArgument `json:",inline"`
	EventcommandId  types.Binary `json:"eventcommand_id"`
}

type EventcommandEnvvar struct {
	CommandEnvvar  `json:",inline"`
	EventcommandId types.Binary `json:"eventcommand_id"`
}

type EventcommandCustomvar struct {
	CustomvarMeta  `json:",inline"`
	EventcommandId types.Binary `json:"eventcommand_id"`
}

type Notificationcommand struct {
	Command `json:",inline"`
}

type NotificationcommandArgument struct {
	CommandArgument       `json:",inline"`
	NotificationcommandId types.Binary `json:"notificationcommand_id"`
}

type NotificationcommandEnvvar struct {
	CommandEnvvar         `json:",inline"`
	NotificationcommandId types.Binary `json:"notificationcommand_id"`
}

type NotificationcommandCustomvar struct {
	CustomvarMeta         `json:",inline"`
	NotificationcommandId types.Binary `json:"notificationcommand_id"`
}

func NewCheckcommand() database.Entity {
	return &Checkcommand{}
}

func NewCheckcommandArgument() database.Entity {
	return &CheckcommandArgument{}
}

func NewCheckcommandEnvvar() database.Entity {
	return &CheckcommandEnvvar{}
}

func NewCheckcommandCustomvar() database.Entity {
	return &CheckcommandCustomvar{}
}

func NewEventcommand() database.Entity {
	return &Eventcommand{}
}

func NewEventcommandArgument() database.Entity {
	return &EventcommandArgument{}
}

func NewEventcommandEnvvar() database.Entity {
	return &EventcommandEnvvar{}
}

func NewEventcommandCustomvar() database.Entity {
	return &EventcommandCustomvar{}
}

func NewNotificationcommand() database.Entity {
	return &Notificationcommand{}
}

func NewNotificationcommandArgument() database.Entity {
	return &NotificationcommandArgument{}
}

func NewNotificationcommandEnvvar() database.Entity {
	return &NotificationcommandEnvvar{}
}

func NewNotificationcommandCustomvar() database.Entity {
	return &NotificationcommandCustomvar{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Command)(nil)
	_ contracts.Initer = (*CommandArgument)(nil)
	_ contracts.Initer = (*Checkcommand)(nil)
	_ contracts.Initer = (*Eventcommand)(nil)
	_ contracts.Initer = (*Notificationcommand)(nil)
)

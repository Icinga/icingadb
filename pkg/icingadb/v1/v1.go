package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
)

var Factories = []contracts.EntityFactoryFunc{
	NewActionUrl,
	NewCheckcommand,
	NewCheckcommandArgument,
	NewCheckcommandCustomvar,
	NewCheckcommandEnvvar,
	NewComment,
	NewDowntime,
	NewEndpoint,
	NewEventcommand,
	NewEventcommandArgument,
	NewEventcommandCustomvar,
	NewEventcommandEnvvar,
	NewHost,
	NewHostCustomvar,
	NewHostState,
	NewHostgroup,
	NewHostgroupCustomvar,
	NewHostgroupMember,
	NewIconImage,
	NewNotesUrl,
	NewNotification,
	NewNotificationcommand,
	NewNotificationcommandArgument,
	NewNotificationcommandCustomvar,
	NewNotificationcommandEnvvar,
	NewNotificationCustomvar,
	NewNotificationRecipient,
	NewNotificationUser,
	NewNotificationUsergroup,
	NewService,
	NewServiceCustomvar,
	NewServiceState,
	NewServicegroup,
	NewServicegroupCustomvar,
	NewServicegroupMember,
	NewTimeperiod,
	NewTimeperiodCustomvar,
	NewTimeperiodOverrideExclude,
	NewTimeperiodOverrideInclude,
	NewTimeperiodRange,
	NewUser,
	NewUserCustomvar,
	NewUsergroup,
	NewUsergroupCustomvar,
	NewUsergroupMember,
	NewZone,
}

// contextKey is an unexported type for context keys defined in this package.
// This prevents collisions with keys defined in other packages.
type contextKey int

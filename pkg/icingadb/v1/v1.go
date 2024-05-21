package v1

import (
	"github.com/icinga/icingadb/pkg/database"
)

var StateFactories = []database.EntityFactoryFunc{NewHostState, NewServiceState}

var ConfigFactories = []database.EntityFactoryFunc{
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

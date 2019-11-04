package configobject

import (
	"github.com/Icinga/icingadb/connection"
)

type ObjectInformation struct {
	ObjectType               string
	RedisKey                 string
	PrimaryMySqlField        string
	HasChecksum              bool
	NotificationListenerType string
	Factory                  connection.RowFactory
	BulkInsertStmt           *connection.BulkInsertStmt
	BulkDeleteStmt           *connection.BulkDeleteStmt
	BulkUpdateStmt           *connection.BulkUpdateStmt
}

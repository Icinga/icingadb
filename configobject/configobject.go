package configobject

import (
	"git.icinga.com/icingadb/icingadb-main/connection"
)

type ObjectInformation struct {
	ObjectType 		string
	RedisKey		string
	HasChecksum		bool
	Factory    		connection.RowFactory
	BulkInsertStmt 	*connection.BulkInsertStmt
	BulkDeleteStmt 	*connection.BulkDeleteStmt
	BulkUpdateStmt 	*connection.BulkUpdateStmt
}
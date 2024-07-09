package mysql

import _ "embed"

//go:embed schema.sql
var Schema string

//go:embed upgrades/1.0.0.sql
var Upgrade100 string

//go:embed upgrades/1.1.1.sql
var Upgrade111 string

//go:embed upgrades/1.2.0.sql
var Upgrade120 string

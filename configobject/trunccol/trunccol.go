package trunccol

import (
	"database/sql/driver"
	"github.com/Icinga/icingadb/utils"
)

type Txtcol string

type Mediumtxtcol string

type Perfcol string

func (msg Txtcol) Value() (driver.Value, error) {
	str, _ := utils.TruncText(string(msg), 65535)

	return str, nil
}

func (msr Mediumtxtcol) Value() (driver.Value, error) {
	str, _ := utils.TruncText(string(msr), 16777215)

	return str, nil
}

func (msg Perfcol) Value() (driver.Value, error) {
	str, _ := utils.TruncPerfData(string(msg), 16777215)

	return str, nil
}

package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"fmt"
)

// Acknowledgement specifies an acknowledgement state (yes, no, sticky).
type AcknowledgementState uint8

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (as *AcknowledgementState) UnmarshalText(bytes []byte) error {
	return as.UnmarshalJSON(bytes)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (as *AcknowledgementState) UnmarshalJSON(data []byte) error {
	var i uint8
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}

	a := AcknowledgementState(i)
	if _, ok := acknowledgementStates[a]; !ok {
		return BadAcknowledgementState{data}
	}

	*as = a
	return nil
}

// Value implements the driver.Valuer interface.
func (as AcknowledgementState) Value() (driver.Value, error) {
	if v, ok := acknowledgementStates[as]; ok {
		return v, nil
	} else {
		return nil, BadAcknowledgementState{as}
	}
}

// BadAcknowledgementState complains about a syntactically, but not semantically valid AcknowledgementState.
type BadAcknowledgementState struct {
	State interface{}
}

// Error implements the error interface.
func (bas BadAcknowledgementState) Error() string {
	return fmt.Sprintf("bad acknowledgement state: %#v", bas.State)
}

// acknowledgementStates maps all valid AcknowledgementState values to their SQL representation.
var acknowledgementStates = map[AcknowledgementState]string{
	0: "n",
	1: "y",
	2: "sticky",
}

// Assert interface compliance.
var (
	_ error                    = BadAcknowledgementState{}
	_ encoding.TextUnmarshaler = (*AcknowledgementState)(nil)
	_ json.Unmarshaler         = (*AcknowledgementState)(nil)
	_ driver.Valuer            = AcknowledgementState(0)
)

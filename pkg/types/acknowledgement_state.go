package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/pkg/errors"
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
		return badAcknowledgementState(data)
	}

	*as = a
	return nil
}

// Value implements the driver.Valuer interface.
func (as AcknowledgementState) Value() (driver.Value, error) {
	if v, ok := acknowledgementStates[as]; ok {
		return v, nil
	} else {
		return nil, badAcknowledgementState(as)
	}
}

// badAcknowledgementState returns an error about a syntactically, but not semantically valid AcknowledgementState.
func badAcknowledgementState(s interface{}) error {
	return errors.Errorf("bad acknowledgement state: %#v", s)
}

// acknowledgementStates maps all valid AcknowledgementState values to their SQL representation.
var acknowledgementStates = map[AcknowledgementState]string{
	0: "n",
	1: "y",
	2: "sticky",
}

// Assert interface compliance.
var (
	_ encoding.TextUnmarshaler = (*AcknowledgementState)(nil)
	_ json.Unmarshaler         = (*AcknowledgementState)(nil)
	_ driver.Valuer            = AcknowledgementState(0)
)

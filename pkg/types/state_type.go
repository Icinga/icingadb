package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"fmt"
)

// StateType specifies a state's hardness.
type StateType uint8

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (st *StateType) UnmarshalText(bytes []byte) error {
	return st.UnmarshalJSON(bytes)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (st *StateType) UnmarshalJSON(data []byte) error {
	var i uint8
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}

	s := StateType(i)
	if _, ok := stateTypes[s]; !ok {
		return BadStateType{data}
	}

	*st = s
	return nil
}

// Value implements the driver.Valuer interface.
func (st StateType) Value() (driver.Value, error) {
	if v, ok := stateTypes[st]; ok {
		return v, nil
	} else {
		return nil, BadStateType{st}
	}
}

// BadStateType complains about a syntactically, but not semantically valid StateType.
type BadStateType struct {
	Type interface{}
}

// Error implements the error interface.
func (bst BadStateType) Error() string {
	return fmt.Sprintf("bad state type: %#v", bst.Type)
}

// stateTypes maps all valid StateType values to their SQL representation.
var stateTypes = map[StateType]string{
	0: "soft",
	1: "hard",
}

// Assert interface compliance.
var (
	_ error                    = BadStateType{}
	_ encoding.TextUnmarshaler = (*StateType)(nil)
	_ json.Unmarshaler         = (*StateType)(nil)
	_ driver.Valuer            = StateType(0)
)

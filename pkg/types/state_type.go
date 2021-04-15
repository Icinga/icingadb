package types

import (
	"database/sql/driver"
	"encoding"
	"fmt"
	"strconv"
)

// StateType specifies a state's hardness.
type StateType uint8

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (st *StateType) UnmarshalText(bytes []byte) error {
	text := string(bytes)

	i, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return err
	}

	s := StateType(i)
	if uint64(s) != i {
		// Truncated due to above cast, obviously too high
		return BadStateType{text}
	}

	if _, ok := stateTypes[s]; !ok {
		return BadStateType{text}
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
	_ driver.Valuer            = StateType(0)
)

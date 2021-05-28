package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"fmt"
	"strconv"
)

// CommentType specifies a comment's origin's kind.
type CommentType uint8

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ct *CommentType) UnmarshalJSON(bytes []byte) error {
	var i uint8
	if err := json.Unmarshal(bytes, &i); err != nil {
		return err
	}

	c := CommentType(i)
	if _, ok := commentTypes[c]; !ok {
		return BadCommentType{bytes}
	}

	*ct = c
	return nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (ct *CommentType) UnmarshalText(bytes []byte) error {
	text := string(bytes)

	i, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return err
	}

	c := CommentType(i)
	if uint64(c) != i {
		// Truncated due to above cast, obviously too high
		return BadCommentType{text}
	}

	if _, ok := commentTypes[c]; !ok {
		return BadCommentType{text}
	}

	*ct = c
	return nil
}

// Value implements the driver.Valuer interface.
func (ct CommentType) Value() (driver.Value, error) {
	if v, ok := commentTypes[ct]; ok {
		return v, nil
	} else {
		return nil, BadCommentType{ct}
	}
}

// BadCommentType complains about a syntactically, but not semantically valid CommentType.
type BadCommentType struct {
	Type interface{}
}

// Error implements the error interface.
func (bct BadCommentType) Error() string {
	return fmt.Sprintf("bad comment type: %#v", bct.Type)
}

// commentTypes maps all valid CommentType values to their SQL representation.
var commentTypes = map[CommentType]string{
	1: "comment",
	4: "ack",
}

// Assert interface compliance.
var (
	_ error                    = BadCommentType{}
	_ json.Unmarshaler         = (*CommentType)(nil)
	_ encoding.TextUnmarshaler = (*CommentType)(nil)
	_ driver.Valuer            = CommentType(0)
)

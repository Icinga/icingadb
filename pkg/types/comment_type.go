package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"github.com/icinga/icingadb/internal"
	"github.com/pkg/errors"
	"strconv"
)

// CommentType specifies a comment's origin's kind.
type CommentType uint8

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ct *CommentType) UnmarshalJSON(bytes []byte) error {
	var i uint8
	if err := internal.UnmarshalJSON(bytes, &i); err != nil {
		return err
	}

	c := CommentType(i)
	if _, ok := commentTypes[c]; !ok {
		return badCommentType(bytes)
	}

	*ct = c
	return nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (ct *CommentType) UnmarshalText(bytes []byte) error {
	text := string(bytes)

	i, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return internal.CantParseUint64(err, text)
	}

	c := CommentType(i)
	if uint64(c) != i {
		// Truncated due to above cast, obviously too high
		return badCommentType(text)
	}

	if _, ok := commentTypes[c]; !ok {
		return badCommentType(text)
	}

	*ct = c
	return nil
}

// Value implements the driver.Valuer interface.
func (ct CommentType) Value() (driver.Value, error) {
	if v, ok := commentTypes[ct]; ok {
		return v, nil
	} else {
		return nil, badCommentType(ct)
	}
}

// badCommentType returns an error about a syntactically, but not semantically valid CommentType.
func badCommentType(t interface{}) error {
	return errors.Errorf("bad comment type: %#v", t)
}

// commentTypes maps all valid CommentType values to their SQL representation.
var commentTypes = map[CommentType]string{
	1: "comment",
	4: "ack",
}

// Assert interface compliance.
var (
	_ json.Unmarshaler         = (*CommentType)(nil)
	_ encoding.TextUnmarshaler = (*CommentType)(nil)
	_ driver.Valuer            = CommentType(0)
)

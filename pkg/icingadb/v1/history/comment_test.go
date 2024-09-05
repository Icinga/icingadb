package history

import (
	"database/sql/driver"
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCommentEventTime_Value(t *testing.T) {
	add := types.UnixMilli(time.Unix(23, 320000000))
	del := types.UnixMilli(time.Unix(42, 240000000))
	expire := types.UnixMilli(time.Unix(1337, 733100000))

	subtests := []struct {
		name   string
		input  *HistoryComment
		output driver.Value
	}{
		{name: "nil-history"},
		{name: "bad-event-type", input: &HistoryComment{
			HistoryMeta: HistoryMeta{EventType: "bad"}, EntryTime: add, RemoveTime: del, ExpireTime: expire,
		}},
		{name: "add", output: int64(23320), input: &HistoryComment{
			HistoryMeta: HistoryMeta{EventType: "comment_add"}, EntryTime: add, RemoveTime: del, ExpireTime: expire,
		}},
		{name: "remove", output: int64(42240), input: &HistoryComment{
			HistoryMeta: HistoryMeta{EventType: "comment_remove"}, EntryTime: add, RemoveTime: del, ExpireTime: expire,
		}},
		{name: "expire", output: int64(1337733), input: &HistoryComment{
			HistoryMeta: HistoryMeta{EventType: "comment_remove"}, EntryTime: add, ExpireTime: expire,
		}},
		{name: "add-nil", input: &HistoryComment{
			HistoryMeta: HistoryMeta{EventType: "comment_add"}, RemoveTime: del, ExpireTime: expire,
		}},
		{name: "expire-nil", input: &HistoryComment{
			HistoryMeta: HistoryMeta{EventType: "comment_remove"}, EntryTime: add,
		}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			v, err := CommentEventTime{st.input}.Value()

			require.NoError(t, err)
			require.Equal(t, st.output, v)
		})
	}
}

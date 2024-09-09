package history

import (
	"database/sql/driver"
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestFlappingEventTime_Value(t *testing.T) {
	s := types.UnixMilli(time.Unix(12, 345000000))
	e := types.UnixMilli(time.Unix(67, 890000000))

	subtests := []struct {
		name   string
		input  *HistoryFlapping
		output driver.Value
	}{
		{name: "nil-history"},
		{
			name:  "bad-event-type",
			input: &HistoryFlapping{HistoryMeta: HistoryMeta{EventType: "bad"}, StartTime: s, EndTime: e},
		},
		{
			name:   "start",
			input:  &HistoryFlapping{HistoryMeta: HistoryMeta{EventType: "flapping_start"}, StartTime: s, EndTime: e},
			output: int64(12345),
		},
		{
			name:   "end",
			input:  &HistoryFlapping{HistoryMeta: HistoryMeta{EventType: "flapping_end"}, StartTime: s, EndTime: e},
			output: int64(67890),
		},
		{
			name:  "start-nil",
			input: &HistoryFlapping{HistoryMeta: HistoryMeta{EventType: "flapping_start"}, EndTime: e},
		},
		{
			name:  "end-nil",
			input: &HistoryFlapping{HistoryMeta: HistoryMeta{EventType: "flapping_end"}, StartTime: s},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			v, err := (FlappingEventTime{History: st.input}).Value()

			require.NoError(t, err)
			require.Equal(t, st.output, v)
		})
	}
}

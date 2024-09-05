package history

import (
	"database/sql/driver"
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestDowntimeEventTime_Value(t *testing.T) {
	start := types.UnixMilli(time.Unix(23, 320000000))
	cancel := types.UnixMilli(time.Unix(42, 240000000))
	end := types.UnixMilli(time.Unix(1337, 733100000))

	f := types.Bool{Bool: false, Valid: true}
	T := types.Bool{Bool: true, Valid: true}

	subtests := []struct {
		name   string
		input  *HistoryDowntime
		output driver.Value
	}{
		{name: "nil-history"},
		{name: "bad-event-type", input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "bad"},
			StartTime:   start, CancelTime: cancel, EndTime: end, HasBeenCancelled: T,
		}},
		{name: "start", output: int64(23320), input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "downtime_start"},
			StartTime:   start, CancelTime: cancel, EndTime: end, HasBeenCancelled: T,
		}},
		{name: "has-been-cancelled-nil", input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "downtime_end"},
			StartTime:   start, CancelTime: cancel, EndTime: end,
		}},
		{name: "has-been-cancelled", output: int64(42240), input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "downtime_end"},
			StartTime:   start, CancelTime: cancel, EndTime: end, HasBeenCancelled: T,
		}},
		{name: "end", output: int64(1337733), input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "downtime_end"},
			StartTime:   start, CancelTime: cancel, EndTime: end, HasBeenCancelled: f,
		}},
		{name: "start-nil", input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "downtime_start"},
			CancelTime:  cancel, EndTime: end, HasBeenCancelled: T,
		}},
		{name: "cancel-time-nil", input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "downtime_end"},
			StartTime:   start, EndTime: end, HasBeenCancelled: T,
		}},
		{name: "end-nil", input: &HistoryDowntime{
			HistoryMeta: HistoryMeta{EventType: "downtime_end"},
			StartTime:   start, CancelTime: cancel, HasBeenCancelled: f,
		}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			v, err := (DowntimeEventTime{History: st.input}).Value()

			require.NoError(t, err)
			require.Equal(t, st.output, v)
		})
	}
}

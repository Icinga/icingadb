package v1

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestStatsMessage_IcingaStatus(t *testing.T) {
	subtests := []struct {
		name   string
		input  StatsMessage
		output IcingaStatus
		error  bool
	}{
		{
			name:  "nil",
			error: true,
		},
		{
			name:  "empty",
			error: true,
		},
		{
			name:  "IcingaApplicationNil",
			input: StatsMessage{"IcingaApplication": nil},
			error: true,
		},
		{
			name:  "IcingaApplicationBool",
			input: StatsMessage{"IcingaApplication": true},
			error: true,
		},
		{
			name:  "IcingaApplicationInt",
			input: StatsMessage{"IcingaApplication": 42},
			error: true,
		},
		{
			name:  "IcingaApplicationNotJSON",
			input: StatsMessage{"IcingaApplication": "not JSON"},
			error: true,
		},
		{
			name:  "IcingaApplicationArray",
			input: StatsMessage{"IcingaApplication": "[]"},
			error: true,
		},
		{
			name:  "IcingaApplicationEmptyJSON",
			input: StatsMessage{"IcingaApplication": "{}"},
		},
		{
			name: "IcingaApplicationWithNodeName",
			input: StatsMessage{
				"IcingaApplication": `{"status":{"icingaapplication":{"app":{"node_name":"foobar"}}}}`,
			},
			output: IcingaStatus{NodeName: "foobar"},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			if output, err := st.input.IcingaStatus(); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, &st.output, output)
			}
		})
	}
}

func TestStatsMessage_Time(t *testing.T) {
	subtests := []struct {
		name   string
		input  StatsMessage
		output types.UnixMilli
		error  bool
	}{
		{
			name:  "nil",
			error: true,
		},
		{
			name:  "empty",
			error: true,
		},
		{
			name:  "timestampNil",
			input: StatsMessage{"timestamp": nil},
			error: true,
		},
		{
			name:  "timestampBool",
			input: StatsMessage{"timestamp": true},
			error: true,
		},
		{
			name:  "timestampInt",
			input: StatsMessage{"timestamp": 42},
			error: true,
		},
		{
			name:  "timestampNotJSON",
			input: StatsMessage{"timestamp": "not JSON"},
			error: true,
		},
		{
			name:  "timestampArray",
			input: StatsMessage{"timestamp": "[]"},
			error: true,
		},
		{
			name:  "timestampNull",
			input: StatsMessage{"timestamp": "null"},
		},
		{
			name:   "timestamp",
			input:  StatsMessage{"timestamp": "42023"},
			output: types.UnixMilli(time.Unix(42, 23000000)),
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			if output, err := st.input.Time(); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, &st.output, output)
			}
		})
	}
}

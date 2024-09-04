package v1

import (
	"github.com/stretchr/testify/require"
	"testing"
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

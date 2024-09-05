package v1

import (
	"database/sql/driver"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddressBin_Value(t *testing.T) {
	subtests := []struct {
		name   string
		input  *Host
		output driver.Value
	}{
		{name: "nil-host"},
		{name: "invalid-address", input: &Host{Address: "invalid"}},
		{name: "IPv6", input: &Host{Address: "2001:db8::"}},
		{name: "IPv4", input: &Host{Address: "192.0.2.0"}, output: []byte{192, 0, 2, 0}},
		{name: "ffff-IPv4", input: &Host{Address: "::ffff:192.0.2.0"}, output: []byte{192, 0, 2, 0}},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			v, err := (AddressBin{Host: st.input}).Value()

			require.NoError(t, err)
			require.Equal(t, st.output, v)
		})
	}
}

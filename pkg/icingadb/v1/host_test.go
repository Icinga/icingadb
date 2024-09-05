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

func TestAddress6Bin_Value(t *testing.T) {
	ffff192020 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255, 192, 0, 2, 0}

	subtests := []struct {
		name   string
		input  *Host
		output driver.Value
	}{
		{name: "nil-host"},
		{name: "invalid-address", input: &Host{Address: "invalid"}},
		{"IPv6", &Host{Address6: "2001:db8::"}, []byte{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "IPv4", input: &Host{Address6: "192.0.2.0"}, output: ffff192020},
		{name: "ffff-IPv4", input: &Host{Address6: "::ffff:192.0.2.0"}, output: ffff192020},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			v, err := (Address6Bin{Host: st.input}).Value()

			require.NoError(t, err)
			require.Equal(t, st.output, v)
		})
	}
}

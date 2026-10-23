package notifications

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckIcinga2Version(t *testing.T) {
	t.Run("Supported", func(t *testing.T) {
		for _, version := range []string{"v2.16.6", "2.16.6", "v2.17.0", "v2.17.1", "v2.18.0-3-g0123abc", "v3.0.0", "v2.17.0-rc1-12-g0123abc", "v2.16.7", "v2.16.6-1-g0123abc"} {
			require.NoErrorf(t, CheckIcinga2Version(version), "version %q", version)
		}
	})

	t.Run("TooOld", func(t *testing.T) {
		for _, version := range []string{"v2.16.5", "v2.15.7", "r2.14.2-1", "v2.15.6", "v2.13.4", "v1.99.99", "v2.16.5-120-g0123abc"} {
			require.ErrorContainsf(t, CheckIcinga2Version(version), "is too old", "version %q", version)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		for _, version := range []string{"", "v2.17", "v2.17.x", "unknown"} {
			require.ErrorContainsf(t, CheckIcinga2Version(version), "unexpected Icinga 2 version", "version %q", version)
		}
	})
}

package notifications

import (
	"fmt"
	"slices"
	"strings"
)

// CheckIcinga2Version returns an error if the given Icinga 2 version is too old to be used with Icinga Notifications.
//
// Older versions don't write all the state metadata to Redis that is required to reconstruct the history event IDs,
// so Icinga DB can't submit correct events with them. This is provided since v2.16.6.
func CheckIcinga2Version(version string) error {
	var v [3]int
	// Icinga 2 reports versions like "v2.16.5", "r2.14.2-1" or "v2.17.0-rc1-12-g0123abc".
	// Anything following the patch level is ignored.
	if _, err := fmt.Sscanf(strings.TrimLeft(version, "vr"), "%d.%d.%d", &v[0], &v[1], &v[2]); err != nil {
		return fmt.Errorf("unexpected Icinga 2 version %q: %w", version, err)
	}

	if slices.Equal(v[:], []int{2, 16, 0}) {
		// All Icinga 2 development versions before 2.17 are technically identified as v2.16.0-N-COMMIT.
		// TODO: Remove sometimes after 2.17 got out.
		return nil
	}

	if slices.Compare(v[:], []int{2, 16, 6}) >= 0 {
		return nil
	}

	return fmt.Errorf("Icinga 2 version %q is too old, v2.16.6 or later is required", version)
}

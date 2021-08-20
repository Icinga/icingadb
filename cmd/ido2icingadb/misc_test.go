package main

import (
	"bytes"
	"testing"
)

func TestMkDeterministicUuid(t *testing.T) {
	if !bytes.Equal(
		mkDeterministicUuid('s', 0x0102030405060708),
		[]byte{'I', 'D', 'O', 's', 'h', 0, 0x40, 1, 0x80, 2, 3, 4, 5, 6, 7, 8},
	) {
		t.Error("got wrong UUID from mkDeterministicUuid(stateHistory, 0x0102030405060708)")
	}
}

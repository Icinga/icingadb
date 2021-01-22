package main

import (
	"github.com/google/uuid"
	"testing"
)

func TestMkDeterministicUuid(t *testing.T) {
	if mkDeterministicUuid(stateHistory, 0x0102030405060708) != (uuid.UUID{
		'I', 'D', 'O', 's', 'h', 0, 0x40, 1, 0x80, 2, 3, 4, 5, 6, 7, 8,
	}) {
		t.Error("got wrong UUID from mkDeterministicUuid(stateHistory, 0x0102030405060708)")
	}
}

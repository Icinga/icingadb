package utils

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSliceSubsets(t *testing.T) {
	data := []string{
		"bla",
		"blub",
		"derp",
	}

	result := SliceSubsets(data...)

	expected := [][]string{
		nil,
		{"bla"},
		{"blub"},
		{"bla", "blub"},
		{"derp"},
		{"bla", "derp"},
		{"blub", "derp"},
		{"bla", "blub", "derp"},
	}

	require.Equal(t, expected, result)

	t.Logf("%#v", result)
}

package utils

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChanFromSlice(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		ch := ChanFromSlice[int](nil)
		require.NotNil(t, ch)
		requireClosedEmpty(t, ch)
	})

	t.Run("Empty", func(t *testing.T) {
		ch := ChanFromSlice([]int{})
		require.NotNil(t, ch)
		requireClosedEmpty(t, ch)
	})

	t.Run("NonEmpty", func(t *testing.T) {
		ch := ChanFromSlice([]int{42, 23, 1337})
		require.NotNil(t, ch)
		requireReceive(t, ch, 42)
		requireReceive(t, ch, 23)
		requireReceive(t, ch, 1337)
		requireClosedEmpty(t, ch)
	})
}

// requireReceive is a helper function to check if a value can immediately be received from a channel.
func requireReceive(t *testing.T, ch <-chan int, expected int) {
	t.Helper()

	select {
	case v, ok := <-ch:
		require.True(t, ok, "receiving should return a value")
		require.Equal(t, expected, v)
	default:
		require.Fail(t, "receiving should not block")
	}
}

// requireReceive is a helper function to check if the channel is closed and empty.
func requireClosedEmpty(t *testing.T, ch <-chan int) {
	t.Helper()

	select {
	case _, ok := <-ch:
		require.False(t, ok, "receiving from channel should not return anything")
	default:
		require.Fail(t, "receiving should not block")
	}
}

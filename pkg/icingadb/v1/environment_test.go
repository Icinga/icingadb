package v1

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestEnvironment_NewContext(t *testing.T) {
	deadline := time.Now().Add(time.Minute)
	parent, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	actual, ok := (*Environment)(nil).NewContext(parent).Deadline()

	require.True(t, ok)
	require.Equal(t, deadline, actual)
}

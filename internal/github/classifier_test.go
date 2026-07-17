package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseReason(t *testing.T) {
	reason, err := ParseReason("off-topic")
	require.NoError(t, err)
	require.Equal(t, "OFF_TOPIC", reason)

	_, err = ParseReason("bad-value")
	require.Error(t, err)
}

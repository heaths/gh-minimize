package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNumber(t *testing.T) {
	number, err := ParseNumber("#123")
	require.NoError(t, err)
	require.Equal(t, 123, number)

	_, err = ParseNumber("abc")
	require.Error(t, err)
}

func TestResolveIssueOrPullRequestNumber(t *testing.T) {
	t.Run("explicit arg", func(t *testing.T) {
		number, err := ResolveIssueOrPullRequestNumber([]string{"42"})
		require.NoError(t, err)
		require.Equal(t, 42, number)
	})

	t.Run("requires explicit issue or pr number", func(t *testing.T) {
		_, err := ResolveIssueOrPullRequestNumber(nil)
		require.ErrorContains(t, err, "expected exactly one issue or pull request number argument")
	})

	t.Run("rejects more than one argument", func(t *testing.T) {
		_, err := ResolveIssueOrPullRequestNumber([]string{"42", "43"})
		require.ErrorContains(t, err, "expected exactly one issue or pull request number argument")
	})
}

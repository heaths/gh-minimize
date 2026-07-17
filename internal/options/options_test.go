package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr string
	}{
		{
			name:  "accepts plain number",
			input: "123",
			want:  123,
		},
		{
			name:  "accepts hash-prefixed number",
			input: "#123",
			want:  123,
		},
		{
			name:    "rejects non-number",
			input:   "abc",
			wantErr: `invalid issue or pull request number "abc"`,
		},
		{
			name:    "rejects hash without digits",
			input:   "#",
			wantErr: `invalid issue or pull request number "#"`,
		},
		{
			name:    "rejects zero",
			input:   "0",
			wantErr: `invalid issue or pull request number "0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			number, err := ParseNumber(tt.input)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, number)
		})
	}
}

func TestResolveIssueOrPullRequestNumber(t *testing.T) {
	t.Run("explicit arg", func(t *testing.T) {
		number, err := ResolveIssueOrPullRequestNumber([]string{"42"})
		require.NoError(t, err)
		require.Equal(t, 42, number)
	})

	t.Run("accepts hash-prefixed arg", func(t *testing.T) {
		number, err := ResolveIssueOrPullRequestNumber([]string{"#42"})
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

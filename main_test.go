package main

import (
	"testing"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/stretchr/testify/require"
)

func TestRun_PrintsCobraError(t *testing.T) {
	streams, _, stdout, stderr := iostreams.Test()

	code := run([]string{"list", "1", "2"}, streams)

	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "accepts at most 1 arg(s), received 2")
}

func TestRun_PrintsInvalidNumberError(t *testing.T) {
	streams, _, stdout, stderr := iostreams.Test()

	code := run([]string{"foo"}, streams)

	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), `invalid issue or pull request number "foo"`)
}

func TestRun_PrintsValidationError(t *testing.T) {
	streams, _, stdout, stderr := iostreams.Test()

	code := run([]string{"123"}, streams)

	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "either --reason or --undo must be provided")
}

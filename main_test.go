package main

import (
	"io"
	"os"
	"testing"

	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/stretchr/testify/require"
)

func TestRun_PrintsCobraError(t *testing.T) {
	terminal, stdout, stderr := testTerminal(t, false, false)

	code := run([]string{"list", "1", "2"}, terminal)

	require.Equal(t, 1, code)
	require.Empty(t, fileString(t, stdout))
	require.Contains(t, fileString(t, stderr), "accepts at most 1 arg(s), received 2")
}

func TestRun_PrintsInvalidNumberError(t *testing.T) {
	terminal, stdout, stderr := testTerminal(t, false, false)

	code := run([]string{"foo"}, terminal)

	require.Equal(t, 1, code)
	require.Empty(t, fileString(t, stdout))
	require.Contains(t, fileString(t, stderr), `invalid issue or pull request number "foo"`)
}

func TestRun_PrintsValidationError(t *testing.T) {
	terminal, stdout, stderr := testTerminal(t, false, false)

	code := run([]string{"123"}, terminal)

	require.Equal(t, 1, code)
	require.Empty(t, fileString(t, stdout))
	require.Contains(t, fileString(t, stderr), "either --reason or --undo must be provided")
}

func testTerminal(t *testing.T, tty bool, colorEnabled bool) (term.Term, *os.File, *os.File) {
	t.Helper()

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	require.NoError(t, err)

	originalStdout, originalStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	t.Cleanup(func() {
		os.Stdout, os.Stderr = originalStdout, originalStderr
		if err := stdout.Close(); err != nil {
			t.Errorf("closing stdout temp file: %v", err)
		}
		if err := stderr.Close(); err != nil {
			t.Errorf("closing stderr temp file: %v", err)
		}
	})

	t.Setenv("GH_FORCE_TTY", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("NO_COLOR", "1")
	if tty {
		t.Setenv("GH_FORCE_TTY", "80")
	}
	if colorEnabled {
		t.Setenv("NO_COLOR", "")
	}

	return term.FromEnv(), stdout, stderr
}

func fileString(t *testing.T, file *os.File) string {
	t.Helper()

	_, err := file.Seek(0, io.SeekStart)
	require.NoError(t, err)
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	return string(data)
}

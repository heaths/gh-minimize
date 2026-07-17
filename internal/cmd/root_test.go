package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"

	ghclient "github.com/heaths/gh-minimize/internal/github"
	"github.com/stretchr/testify/require"
)

type mockService struct {
	comments          []ghclient.Comment
	findErr           error
	minimizeErrByID   map[string]error
	unminimizeErrByID map[string]error
	minimized         []string
	unminimized       []string
}

func (m *mockService) FindIssueOrPullRequestComments(owner, repo string, number int) ([]ghclient.Comment, error) {
	return m.comments, m.findErr
}

func (m *mockService) MinimizeComment(id, classifier string) error {
	if err := m.minimizeErrByID[id]; err != nil {
		return err
	}
	m.minimized = append(m.minimized, fmt.Sprintf("%s:%s", id, classifier))
	return nil
}

func (m *mockService) UnminimizeComment(id string) error {
	if err := m.unminimizeErrByID[id]; err != nil {
		return err
	}
	m.unminimized = append(m.unminimized, id)
	return nil
}

func TestValidateFlags(t *testing.T) {
	tests := []struct {
		name    string
		opts    rootOptions
		args    []string
		wantErr string
	}{
		{
			name:    "requires reason or undo",
			opts:    rootOptions{},
			wantErr: "either --reason or --undo",
		},
		{
			name: "id cannot combine with search args",
			opts: rootOptions{
				id:      "id",
				reason:  "abuse",
				authors: []string{"octocat"},
			},
			wantErr: "--id cannot be used",
		},
		{
			name: "requires search filters",
			opts: rootOptions{
				reason: "abuse",
			},
			wantErr: "at least one of --author or --body-grep",
		},
		{
			name: "requires issue or pr number without id",
			opts: rootOptions{
				reason:  "abuse",
				authors: []string{"octocat"},
			},
			wantErr: "exactly one issue or pull request number argument is required",
		},
		{
			name: "accepts search with issue or pr number",
			opts: rootOptions{
				reason:  "abuse",
				authors: []string{"octocat", "hubot"},
			},
			args: []string{"123"},
		},
		{
			name: "accepts id minimize",
			opts: rootOptions{
				id:     "node",
				reason: "abuse",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlags(&tt.opts, tt.args)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestNew_AuthorFlagSupportsMultipleSwitches(t *testing.T) {
	cmd := New()

	err := cmd.ParseFlags([]string{"--author", "octocat", "--author", "hubot"})
	require.NoError(t, err)

	authors, err := cmd.Flags().GetStringArray("author")
	require.NoError(t, err)
	require.Equal(t, []string{"octocat", "hubot"}, authors)
}

func TestNew_UsesGhCommandNameWhenRunningUnderGh(t *testing.T) {
	oldExecutableName := executableName
	t.Cleanup(func() {
		executableName = oldExecutableName
	})
	t.Setenv("GH_EXTENSION", "minimize")
	executableName = func() string { return "gh-minimize" }

	cmd := New()

	require.Equal(t, "gh minimize [issue-or-pr-number]", cmd.Use)
	require.Contains(t, cmd.Example, "$ gh minimize --id MDEyOklzc3VlQ29tbWVudDE= --reason off-topic")
}

func TestNew_UsesExecutableNameOutsideGh(t *testing.T) {
	oldExecutableName := executableName
	t.Cleanup(func() {
		executableName = oldExecutableName
	})
	require.NoError(t, os.Unsetenv("GH_EXTENSION"))
	executableName = func() string { return "gh-minimize" }

	cmd := New()

	require.Equal(t, "gh-minimize [issue-or-pr-number]", cmd.Use)
	require.Contains(t, cmd.Example, "$ gh-minimize --id MDEyOklzc3VlQ29tbWVudDE= --reason off-topic")
}

func TestFilterCommentIDs(t *testing.T) {
	comments := []ghclient.Comment{
		{ID: "1", Author: ghclient.Actor{Login: "octocat"}, Body: "hello world", IsMinimized: false},
		{ID: "2", Author: ghclient.Actor{Login: "octocat"}, Body: "old context", IsMinimized: true},
		{ID: "3", Author: ghclient.Actor{Login: "hubot"}, Body: "old context", IsMinimized: false},
		{ID: "4", Author: ghclient.Actor{Login: "MONA"}, Body: "old context", IsMinimized: false},
	}

	re := regexp.MustCompile("old")
	gotMinimize := filterCommentIDs(comments, []string{"octocat", "mona"}, re, false)
	require.Equal(t, []string{"4"}, gotMinimize)

	gotUndo := filterCommentIDs(comments, []string{"octocat", "hubot"}, re, true)
	require.Equal(t, []string{"2"}, gotUndo)
}

func TestApplyAction_Minimize(t *testing.T) {
	out := &bytes.Buffer{}
	mock := &mockService{}
	opts := &rootOptions{
		reason: "off-topic",
		stdout: out,
		client: mock,
	}

	err := applyAction(opts, []string{"a", "b"})
	require.NoError(t, err)
	require.Equal(t, []string{"a:OFF_TOPIC", "b:OFF_TOPIC"}, mock.minimized)
	require.Contains(t, out.String(), "Minimized 2 comment(s).")
}

func TestApplyAction_UnminimizeError(t *testing.T) {
	out := &bytes.Buffer{}
	mock := &mockService{
		unminimizeErrByID: map[string]error{
			"a": errors.New("boom"),
		},
	}
	opts := &rootOptions{
		undo:   true,
		stdout: out,
		client: mock,
	}

	err := applyAction(opts, []string{"a"})
	require.ErrorContains(t, err, "failed to update comment a")
}

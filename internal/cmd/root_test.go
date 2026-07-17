package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/cli/cli/v2/pkg/iostreams"
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
				id:     "id",
				reason: "abuse",
				filterOptions: filterOptions{
					authors: []string{"octocat"},
				},
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
				reason: "abuse",
				filterOptions: filterOptions{
					authors: []string{"octocat"},
				},
			},
			wantErr: "exactly one issue or pull request number argument is required",
		},
		{
			name: "accepts search with issue or pr number",
			opts: rootOptions{
				reason: "abuse",
				filterOptions: filterOptions{
					authors: []string{"octocat", "hubot"},
				},
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

func TestNew_RepoFlagIsPersistent(t *testing.T) {
	cmd := New()

	require.NotNil(t, cmd.PersistentFlags().Lookup("repo"))

	listCmd, _, err := cmd.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, listCmd.InheritedFlags().Lookup("repo"))
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
		{ID: "1", Author: "octocat", Body: "hello world", IsMinimized: false},
		{ID: "2", Author: "octocat", Body: "old context", IsMinimized: true},
		{ID: "3", Author: "hubot", Body: "old context", IsMinimized: false},
		{ID: "4", Author: "MONA", Body: "old context", IsMinimized: false},
	}

	re := regexp.MustCompile("old")
	gotMinimize := filterCommentIDs(filterComments(comments, []string{"octocat", "mona"}, re), false)
	require.Equal(t, []string{"4"}, gotMinimize)

	gotUndo := filterCommentIDs(filterComments(comments, []string{"octocat", "hubot"}, re), true)
	require.Equal(t, []string{"2"}, gotUndo)
}

func TestApplyAction_Minimize(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	mock := &mockService{}
	opts := &rootOptions{
		reason: "off-topic",
		commonOptions: commonOptions{
			io:     io,
			client: mock,
		},
	}

	err := applyAction(opts, []string{"a", "b"})
	require.NoError(t, err)
	require.Equal(t, []string{"a:OFF_TOPIC", "b:OFF_TOPIC"}, mock.minimized)
	require.Contains(t, out.String(), "Minimized 2 comment(s).")
}

func TestApplyAction_UnminimizeError(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	mock := &mockService{
		unminimizeErrByID: map[string]error{
			"a": errors.New("boom"),
		},
	}
	opts := &rootOptions{
		undo: true,
		commonOptions: commonOptions{
			io:     io,
			client: mock,
		},
	}

	err := applyAction(opts, []string{"a"})
	require.ErrorContains(t, err, "failed to update comment a")
}

func TestRunList_DefaultOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &listOptions{
		commonOptions: commonOptions{
			io:     io,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{
						ID:              "1",
						Author:          "octocat",
						Body:            "hello",
						IsMinimized:     true,
						MinimizedReason: "OUTDATED",
					},
				},
			},
		},
	}

	err := runList(opts, []string{"123"})
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"1","author":"octocat","body":"hello","isMinimized":true,"minimizedReason":"OUTDATED"}]`, out.String())
}

func TestRunList_JQOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &listOptions{
		jqExpression: ".[].author",
		commonOptions: commonOptions{
			io:     io,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{
						ID:              "1",
						Author:          "octocat",
						Body:            "hello",
						IsMinimized:     true,
						MinimizedReason: "OUTDATED",
					},
				},
			},
		},
	}

	err := runList(opts, []string{"123"})
	require.NoError(t, err)
	require.Equal(t, "octocat\n", out.String())
}

func TestRunList_SelectedJSONFields(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &listOptions{
		jsonFields: "id,author",
		commonOptions: commonOptions{
			io:     io,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{
						ID:              "1",
						Author:          "octocat",
						Body:            "hello",
						IsMinimized:     true,
						MinimizedReason: "OUTDATED",
					},
				},
			},
		},
	}

	err := runList(opts, []string{"123"})
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"1","author":"octocat"}]`, out.String())
}

func TestRunList_FilteredOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &listOptions{
		filterOptions: filterOptions{
			authors:  []string{"hubot"},
			bodyGrep: "old",
		},
		commonOptions: commonOptions{
			io:     io,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{ID: "1", Author: "octocat", Body: "old context"},
					{ID: "2", Author: "hubot", Body: "old context"},
					{ID: "3", Author: "hubot", Body: "new context"},
				},
			},
		},
	}

	err := runList(opts, []string{"123"})
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"2","author":"hubot","body":"old context","isMinimized":false,"minimizedReason":""}]`, out.String())
}

func TestWriteCommentOutput_PrettyPrintsJSON(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(true)
	io.SetColorEnabled(false)

	err := writeCommentOutput(&listOptions{commonOptions: commonOptions{io: io}}, []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "hello"},
	})
	require.NoError(t, err)
	require.Contains(t, out.String(), "\n  {\n")
	require.Contains(t, out.String(), `"author": "octocat"`)
}

func TestWriteCommentOutput_DoesNotPrettyPrintTemplate(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(true)
	io.SetColorEnabled(false)

	err := writeCommentOutput(&listOptions{
		commonOptions: commonOptions{io: io},
		tmpl:          "{{range .}}{{.author}}{{end}}",
	}, []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "hello"},
	})
	require.NoError(t, err)
	require.Equal(t, "octocat", out.String())
}

func TestLoadFilteredComments_InvalidRegex(t *testing.T) {
	_, err := loadFilteredComments(&mockService{}, "OWNER/REPO", []string{"123"}, nil, "[")
	require.ErrorContains(t, err, "invalid --body-grep regex")
}

func TestLoadFilteredComments_FiltersPageableResults(t *testing.T) {
	comments, err := loadFilteredComments(&mockService{
		comments: []ghclient.Comment{
			{ID: "1", Author: "octocat", Body: "keep this"},
			{ID: "2", Author: "hubot", Body: "drop this"},
			{ID: "3", Author: "octocat", Body: "keep that"},
		},
	}, "OWNER/REPO", []string{"123"}, []string{"octocat"}, "keep")
	require.NoError(t, err)
	require.Equal(t, []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "keep this"},
		{ID: "3", Author: "octocat", Body: "keep that"},
	}, comments)
}

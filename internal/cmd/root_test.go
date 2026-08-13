package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"testing"

	"github.com/cli/go-gh/v2/pkg/term"
	ghclient "github.com/heaths/gh-minimize/internal/github"
	"github.com/spf13/cobra"
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

type fakeProgressIndicator struct {
	started int
	stopped int
	onStop  func()
}

func (f *fakeProgressIndicator) Start() {
	f.started++
}

func (f *fakeProgressIndicator) Stop() {
	f.stopped++
	if f.onStop != nil {
		f.onStop()
	}
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
			wantErr: "at least one of --author or --grep",
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
			name: "accepts search without issue or pr number",
			opts: rootOptions{
				reason: "abuse",
				filterOptions: filterOptions{
					authors: []string{"octocat", "hubot"},
				},
			},
		},
		{
			name: "accepts id minimize",
			opts: rootOptions{
				id:     "node",
				reason: "abuse",
			},
		},
		{
			name: "id cannot combine with grep",
			opts: rootOptions{
				id:     "id",
				reason: "abuse",
				filterOptions: filterOptions{
					grep: "old",
				},
			},
			wantErr: "--id cannot be used",
		},
		{
			name: "id cannot combine with invert-grep",
			opts: rootOptions{
				id:     "id",
				reason: "abuse",
				filterOptions: filterOptions{
					invertGrep: true,
				},
			},
			wantErr: "--id cannot be used",
		},
		{
			name: "accepts grep alone",
			opts: rootOptions{
				reason: "abuse",
				filterOptions: filterOptions{
					grep: "old",
				},
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

func TestNew_GrepAndInvertGrepFlags(t *testing.T) {
	cmd := New()

	err := cmd.ParseFlags([]string{"--grep", "old.*context", "--invert-grep"})
	require.NoError(t, err)

	grep, err := cmd.Flags().GetString("grep")
	require.NoError(t, err)
	require.Equal(t, "old.*context", grep)

	invertGrep, err := cmd.Flags().GetBool("invert-grep")
	require.NoError(t, err)
	require.True(t, invertGrep)
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

func TestPositionalIssueOrPullRequestArgs(t *testing.T) {
	tests := []struct {
		name     string
		required bool
		args     []string
		wantErr  string
	}{
		{
			name:     "optional accepts plain number",
			required: false,
			args:     []string{"42"},
		},
		{
			name:     "optional accepts hash-prefixed number",
			required: false,
			args:     []string{"#42"},
		},
		{
			name:     "required accepts plain number",
			required: true,
			args:     []string{"42"},
		},
		{
			name:     "required accepts hash-prefixed number",
			required: true,
			args:     []string{"#42"},
		},
		{
			name:     "required rejects missing arg",
			required: true,
			wantErr:  "accepts 1 arg(s), received 0",
		},
		{
			name:     "rejects non-number",
			required: false,
			args:     []string{"foo"},
			wantErr:  `invalid issue or pull request number "foo"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := positionalIssueOrPullRequestArgs(tt.required)(&cobra.Command{}, tt.args)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestFilterCommentIDs(t *testing.T) {
	comments := []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "hello world", IsMinimized: false},
		{ID: "2", Author: "octocat", Body: "old context", IsMinimized: true},
		{ID: "3", Author: "hubot", Body: "old context", IsMinimized: false},
		{ID: "4", Author: "MONA", Body: "old context", IsMinimized: false},
	}

	re := regexp.MustCompile("old")
	gotMinimize := filterCommentIDs(filterComments(comments, []string{"octocat", "mona"}, re, false), false)
	require.Equal(t, []string{"4"}, gotMinimize)

	gotUndo := filterCommentIDs(filterComments(comments, []string{"octocat", "hubot"}, re, false), true)
	require.Equal(t, []string{"2"}, gotUndo)
}

func TestFilterComments_InvertGrep(t *testing.T) {
	comments := []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "hello world"},
		{ID: "2", Author: "octocat", Body: "old context"},
	}

	re := regexp.MustCompile("old")
	got := filterComments(comments, nil, re, true)
	require.Equal(t, []ghclient.Comment{{ID: "1", Author: "octocat", Body: "hello world"}}, got)
}

func TestFilterComments_InvertGrepNoEffectWithoutGrep(t *testing.T) {
	comments := []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "hello world"},
		{ID: "2", Author: "octocat", Body: "old context"},
	}

	got := filterComments(comments, nil, nil, true)
	require.Equal(t, comments, got)
}

func TestRun_MinimizeSkipsAlreadyMinimized(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	indicator := &fakeProgressIndicator{}
	oldNewProgressIndicator := newProgressIndicator
	t.Cleanup(func() {
		newProgressIndicator = oldNewProgressIndicator
	})
	newProgressIndicator = func(io.Writer) progressIndicator { return indicator }
	mock := &mockService{
		comments: []ghclient.Comment{
			{ID: "1", Author: "octocat", Body: "old context", IsMinimized: false},
			{ID: "2", Author: "octocat", Body: "old context", IsMinimized: true},
		},
	}
	opts := &rootOptions{
		reason: "outdated",
		filterOptions: filterOptions{
			authors: []string{"octocat"},
		},
		commonOptions: commonOptions{
			term:   terminal,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: mock,
		},
	}

	err := run(opts, []string{"123"})
	require.NoError(t, err)
	require.Equal(t, []string{"1:OUTDATED"}, mock.minimized)
	requireStatusLine(t, terminal, stdout, terminal.ColorScheme().SuccessIcon(), "Minimized 1 comment")
	require.Equal(t, 2, indicator.started)
	require.Equal(t, 2, indicator.stopped)
}

func TestRun_UnminimizeSkipsAlreadyUnminimized(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	mock := &mockService{
		comments: []ghclient.Comment{
			{ID: "1", Author: "octocat", Body: "old context", IsMinimized: true},
			{ID: "2", Author: "octocat", Body: "old context", IsMinimized: false},
		},
	}
	opts := &rootOptions{
		undo: true,
		filterOptions: filterOptions{
			authors: []string{"octocat"},
		},
		commonOptions: commonOptions{
			term:   terminal,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: mock,
		},
	}

	err := run(opts, []string{"123"})
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, mock.unminimized)
	requireStatusLine(t, terminal, stdout, terminal.ColorScheme().SuccessIcon(), "Unminimized 1 comment")
}

func TestApplyAction_Minimize(t *testing.T) {
	tests := []struct {
		name         string
		colorEnabled bool
	}{
		{name: "color disabled", colorEnabled: false},
		{name: "color enabled", colorEnabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminal, stdout, _ := testTerminal(t, true, tt.colorEnabled)
			indicator := &fakeProgressIndicator{}
			oldNewProgressIndicator := newProgressIndicator
			t.Cleanup(func() {
				newProgressIndicator = oldNewProgressIndicator
			})
			newProgressIndicator = func(io.Writer) progressIndicator { return indicator }
			mock := &mockService{}
			opts := &rootOptions{
				reason: "off-topic",
				commonOptions: commonOptions{
					term:   terminal,
					client: mock,
				},
			}

			err := applyAction(opts, []string{"a", "b"})
			require.NoError(t, err)
			require.Equal(t, []string{"a:OFF_TOPIC", "b:OFF_TOPIC"}, mock.minimized)
			requireStatusLine(t, terminal, stdout, terminal.ColorScheme().SuccessIcon(), "Minimized 2 comments")
			require.Equal(t, 1, indicator.started)
			require.Equal(t, 1, indicator.stopped)
		})
	}
}

func TestApplyAction_StopsSpinnerBeforePrintingSummary(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	indicator := &fakeProgressIndicator{}
	oldNewProgressIndicator := newProgressIndicator
	t.Cleanup(func() {
		newProgressIndicator = oldNewProgressIndicator
	})
	newProgressIndicator = func(io.Writer) progressIndicator { return indicator }
	indicator.onStop = func() {
		require.Empty(t, fileString(t, stdout))
	}
	opts := &rootOptions{
		reason: "off-topic",
		commonOptions: commonOptions{
			term:   terminal,
			client: &mockService{},
		},
	}

	err := applyAction(opts, []string{"a"})
	require.NoError(t, err)
	requireStatusLine(t, terminal, stdout, terminal.ColorScheme().SuccessIcon(), "Minimized 1 comment")
	require.Equal(t, 1, indicator.started)
	require.Equal(t, 1, indicator.stopped)
}

func TestApplyAction_NoMatchingCommentsUsesWarningIcon(t *testing.T) {
	tests := []struct {
		name         string
		colorEnabled bool
	}{
		{name: "color disabled", colorEnabled: false},
		{name: "color enabled", colorEnabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminal, stdout, _ := testTerminal(t, true, tt.colorEnabled)
			opts := &rootOptions{
				reason: "off-topic",
				commonOptions: commonOptions{
					term:   terminal,
					client: &mockService{},
				},
			}

			err := applyAction(opts, nil)
			require.NoError(t, err)
			requireStatusLine(t, terminal, stdout, terminal.ColorScheme().WarningIcon(), "No matching comments found")
		})
	}
}

func TestApplyAction_UnminimizeError(t *testing.T) {
	terminal, _, _ := testTerminal(t, false, false)
	indicator := &fakeProgressIndicator{}
	oldNewProgressIndicator := newProgressIndicator
	t.Cleanup(func() {
		newProgressIndicator = oldNewProgressIndicator
	})
	newProgressIndicator = func(io.Writer) progressIndicator { return indicator }
	mock := &mockService{
		unminimizeErrByID: map[string]error{
			"a": errors.New("boom"),
		},
	}
	opts := &rootOptions{
		undo: true,
		commonOptions: commonOptions{
			term:   terminal,
			client: mock,
		},
	}

	err := applyAction(opts, []string{"a"})
	require.ErrorContains(t, err, "failed to update comment a")
	require.Equal(t, 1, indicator.started)
	require.Equal(t, 1, indicator.stopped)
}

func TestRunList_DefaultOutput(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	opts := &listOptions{
		commonOptions: commonOptions{
			term:   terminal,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{
						ID:              "1",
						Author:          "octocat",
						AuthorType:      "user",
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
	require.JSONEq(t, `[{"id":"1","author":"octocat","authorType":"user","body":"hello","isMinimized":true,"minimizedReason":"OUTDATED"}]`, fileString(t, stdout))
}

func TestRunList_JQOutput(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	opts := &listOptions{
		jqExpression: ".[].author",
		commonOptions: commonOptions{
			term:   terminal,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{
						ID:              "1",
						Author:          "octocat",
						AuthorType:      "user",
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
	require.Equal(t, "octocat\n", fileString(t, stdout))
}

func TestRunList_SelectedJSONFields(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	opts := &listOptions{
		jsonFields: "id,author",
		commonOptions: commonOptions{
			term:   terminal,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{
						ID:              "1",
						Author:          "octocat",
						AuthorType:      "user",
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
	require.JSONEq(t, `[{"id":"1","author":"octocat"}]`, fileString(t, stdout))
}

func TestRunList_SelectedJSONFieldsAuthorType(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	opts := &listOptions{
		jsonFields: "id,authorType",
		commonOptions: commonOptions{
			term:   terminal,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{
						ID:         "1",
						Author:     "dependabot[bot]",
						AuthorType: "bot",
						Body:       "hello",
					},
				},
			},
		},
	}

	err := runList(opts, []string{"123"})
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"1","authorType":"bot"}]`, fileString(t, stdout))
}

func TestRunList_FilteredOutput(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, false, false)
	opts := &listOptions{
		filterOptions: filterOptions{
			authors: []string{"hubot"},
			grep:    "old",
		},
		commonOptions: commonOptions{
			term:   terminal,
			global: &globalOptions{repo: "OWNER/REPO"},
			client: &mockService{
				comments: []ghclient.Comment{
					{ID: "1", Author: "octocat", Body: "old context"},
					{ID: "2", Author: "hubot", AuthorType: "bot", Body: "old context"},
					{ID: "3", Author: "hubot", Body: "new context"},
				},
			},
		},
	}

	err := runList(opts, []string{"123"})
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"2","author":"hubot","authorType":"bot","body":"old context","isMinimized":false,"minimizedReason":""}]`, fileString(t, stdout))
}

func TestWriteCommentOutput_PrettyPrintsJSON(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, true, false)

	err := writeCommentOutput(&listOptions{commonOptions: commonOptions{term: terminal}}, []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "hello"},
	})
	require.NoError(t, err)
	output := fileString(t, stdout)
	require.Contains(t, output, "\n  {\n")
	require.Contains(t, output, `"author": "octocat"`)
}

func TestWriteCommentOutput_DoesNotPrettyPrintTemplate(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, true, false)

	err := writeCommentOutput(&listOptions{
		commonOptions: commonOptions{term: terminal},
		tmpl:          "{{range .}}{{.author}}{{end}}",
	}, []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "hello"},
	})
	require.NoError(t, err)
	require.Equal(t, "octocat", fileString(t, stdout))
}

func TestWriteCommentOutput_TemplateCanAccessAuthorType(t *testing.T) {
	terminal, stdout, _ := testTerminal(t, true, false)

	err := writeCommentOutput(&listOptions{
		commonOptions: commonOptions{term: terminal},
		tmpl:          "{{range .}}{{.authorType}}{{end}}",
	}, []ghclient.Comment{
		{ID: "1", Author: "dependabot[bot]", AuthorType: "bot", Body: "hello"},
	})
	require.NoError(t, err)
	require.Equal(t, "bot", fileString(t, stdout))
}

func TestLoadFilteredComments_InvalidRegex(t *testing.T) {
	_, err := loadFilteredComments(&mockService{}, nil, "OWNER/REPO", []string{"123"}, nil, "[", false)
	require.ErrorContains(t, err, "invalid --grep regex")
}

func TestLoadFilteredComments_FiltersPageableResults(t *testing.T) {
	indicator := &fakeProgressIndicator{}
	oldNewProgressIndicator := newProgressIndicator
	t.Cleanup(func() {
		newProgressIndicator = oldNewProgressIndicator
	})
	newProgressIndicator = func(io.Writer) progressIndicator { return indicator }

	comments, err := loadFilteredComments(&mockService{
		comments: []ghclient.Comment{
			{ID: "1", Author: "octocat", Body: "keep this"},
			{ID: "2", Author: "hubot", Body: "drop this"},
			{ID: "3", Author: "octocat", Body: "keep that"},
		},
	}, nil, "OWNER/REPO", []string{"123"}, []string{"octocat"}, "keep", false)
	require.NoError(t, err)
	require.Equal(t, []ghclient.Comment{
		{ID: "1", Author: "octocat", Body: "keep this"},
		{ID: "3", Author: "octocat", Body: "keep that"},
	}, comments)
	require.Equal(t, 1, indicator.started)
	require.Equal(t, 1, indicator.stopped)
}

func TestLoadFilteredComments_InvertGrep(t *testing.T) {
	indicator := &fakeProgressIndicator{}
	oldNewProgressIndicator := newProgressIndicator
	t.Cleanup(func() {
		newProgressIndicator = oldNewProgressIndicator
	})
	newProgressIndicator = func(io.Writer) progressIndicator { return indicator }

	comments, err := loadFilteredComments(&mockService{
		comments: []ghclient.Comment{
			{ID: "1", Author: "octocat", Body: "keep this"},
			{ID: "2", Author: "hubot", Body: "drop this"},
		},
	}, nil, "OWNER/REPO", []string{"123"}, nil, "keep", true)
	require.NoError(t, err)
	require.Equal(t, []ghclient.Comment{
		{ID: "2", Author: "hubot", Body: "drop this"},
	}, comments)
}

func TestLoadFilteredComments_QueryErrorStopsSpinner(t *testing.T) {
	indicator := &fakeProgressIndicator{}
	oldNewProgressIndicator := newProgressIndicator
	t.Cleanup(func() {
		newProgressIndicator = oldNewProgressIndicator
	})
	newProgressIndicator = func(io.Writer) progressIndicator { return indicator }

	_, err := loadFilteredComments(&mockService{findErr: errors.New("boom")}, nil, "OWNER/REPO", []string{"123"}, nil, "", false)
	require.ErrorContains(t, err, "failed to find comments: boom")
	require.Equal(t, 1, indicator.started)
	require.Equal(t, 1, indicator.stopped)
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

func requireStatusLine(t *testing.T, terminal term.Term, file *os.File, icon, message string) {
	t.Helper()

	require.Equal(t, fmt.Sprintf("%s %s\n", icon, message), fileString(t, file))
}

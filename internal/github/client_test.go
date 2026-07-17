package github

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeGQLClient struct {
	do func(query string, vars map[string]interface{}, response interface{}) error
}

func (f *fakeGQLClient) Do(query string, vars map[string]interface{}, response interface{}) error {
	return f.do(query, vars, response)
}

func TestFindIssueOrPullRequestComments(t *testing.T) {
	calls := 0
	client := NewWithClient(&fakeGQLClient{
		do: func(query string, vars map[string]interface{}, response interface{}) error {
			calls++
			resp := response.(*struct {
				Repository struct {
					IssueOrPullRequest *struct {
						Comments commentConnection `json:"comments"`
					} `json:"issueOrPullRequest"`
				} `json:"repository"`
			})

			resp.Repository.IssueOrPullRequest = &struct {
				Comments commentConnection `json:"comments"`
			}{}
			resp.Repository.IssueOrPullRequest.Comments.Nodes = []Comment{
				{ID: "1"},
				{ID: "2"},
			}
			resp.Repository.IssueOrPullRequest.Comments.PageInfo.HasNextPage = false
			return nil
		},
	})

	comments, err := client.FindIssueOrPullRequestComments("owner", "repo", 1)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Len(t, comments, 2)
}

func TestMinimizeComment(t *testing.T) {
	client := NewWithClient(&fakeGQLClient{
		do: func(query string, vars map[string]interface{}, response interface{}) error {
			resp := response.(*struct {
				MinimizeComment struct {
					MinimizedComment struct {
						IsMinimized bool `json:"isMinimized"`
					} `json:"minimizedComment"`
				} `json:"minimizeComment"`
			})
			resp.MinimizeComment.MinimizedComment.IsMinimized = true
			return nil
		},
	})

	require.NoError(t, client.MinimizeComment("id", "SPAM"))
}

func TestUnminimizeCommentError(t *testing.T) {
	client := NewWithClient(&fakeGQLClient{
		do: func(query string, vars map[string]interface{}, response interface{}) error {
			return errors.New("boom")
		},
	})

	err := client.UnminimizeComment("id")
	require.ErrorContains(t, err, "boom")
}

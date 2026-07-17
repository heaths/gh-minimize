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
			switch query {
			case queryComments:
				resp := response.(*commentsQueryResponse)

				resp.Repository.IssueOrPullRequest = &struct {
					TypeName string            `json:"__typename"`
					Comments commentConnection `json:"comments"`
				}{
					TypeName: "Issue",
				}
				resp.Repository.IssueOrPullRequest.Comments.Nodes = []graphqlComment{
					{ID: "1", Author: &graphqlActor{Login: "octocat"}, BodyText: "hello"},
					{ID: "2", BodyText: "goodbye"},
				}
				resp.Repository.IssueOrPullRequest.Comments.PageInfo.HasNextPage = false
				return nil
			default:
				return errors.New("unexpected query")
			}
		},
	})

	comments, err := client.FindIssueOrPullRequestComments("owner", "repo", 1)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Len(t, comments, 2)
	require.Equal(t, Comment{ID: "1", Author: "octocat", Body: "hello"}, comments[0])
	require.Equal(t, Comment{ID: "2", Body: "goodbye"}, comments[1])
}

func TestParseCommentFields(t *testing.T) {
	fields, err := ParseCommentFields("id, author,body")
	require.NoError(t, err)
	require.Equal(t, []string{"id", "author", "body"}, fields)
}

func TestExportComments_InvalidField(t *testing.T) {
	_, err := ExportComments([]Comment{{ID: "1"}}, []string{"bogus"})
	require.ErrorContains(t, err, `unknown JSON field "bogus"`)
}

func TestFindIssueOrPullRequestComments_IncludesReviewComments(t *testing.T) {
	client := NewWithClient(&fakeGQLClient{
		do: func(query string, vars map[string]interface{}, response interface{}) error {
			switch query {
			case queryComments:
				resp := response.(*commentsQueryResponse)
				resp.Repository.IssueOrPullRequest = &struct {
					TypeName string            `json:"__typename"`
					Comments commentConnection `json:"comments"`
				}{
					TypeName: "PullRequest",
				}
				resp.Repository.IssueOrPullRequest.Comments.Nodes = []graphqlComment{
					{ID: "1", Author: &graphqlActor{Login: "octocat"}, BodyText: "timeline"},
				}
				return nil
			case queryReviewThreads:
				resp := response.(*reviewThreadsQueryResponse)
				resp.Repository.PullRequest = &struct {
					ReviewThreads reviewThreadConnection `json:"reviewThreads"`
				}{}
				resp.Repository.PullRequest.ReviewThreads.Nodes = []reviewThread{
					{
						ID: "thread-1",
						Comments: commentConnection{
							Nodes: []graphqlComment{
								{ID: "2", Author: &graphqlActor{Login: "hubot"}, BodyText: "review"},
							},
						},
					},
				}
				return nil
			default:
				return errors.New("unexpected query")
			}
		},
	})

	comments, err := client.FindIssueOrPullRequestComments("owner", "repo", 1)
	require.NoError(t, err)
	require.Equal(t, []Comment{
		{ID: "1", Author: "octocat", Body: "timeline"},
		{ID: "2", Author: "hubot", Body: "review"},
	}, comments)
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

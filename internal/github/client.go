package github

import (
	"fmt"
	"io"

	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/api"
)

type GraphQLClient interface {
	Do(query string, variables map[string]interface{}, response interface{}) error
}

type Client struct {
	gql GraphQLClient
}

func New(log io.Writer) (*Client, error) {
	gql, err := gh.GQLClient(&api.ClientOptions{Log: log})
	if err != nil {
		return nil, err
	}

	return NewWithClient(gql), nil
}

func NewWithClient(gql GraphQLClient) *Client {
	return &Client{gql: gql}
}

type Comment struct {
	ID              string `json:"id"`
	Body            string `json:"bodyText"`
	Author          Actor  `json:"author"`
	IsMinimized     bool   `json:"isMinimized"`
	MinimizedReason string `json:"minimizedReason"`
}

type Actor struct {
	Login string `json:"login"`
}

func (c *Client) FindIssueOrPullRequestComments(owner, repo string, number int) ([]Comment, error) {
	var (
		comments  []Comment
		endCursor string
	)

	for {
		vars := map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": number,
		}
		if endCursor != "" {
			vars["endCursor"] = endCursor
		}

		var response struct {
			Repository struct {
				IssueOrPullRequest *struct {
					Comments commentConnection `json:"comments"`
				} `json:"issueOrPullRequest"`
			} `json:"repository"`
		}

		if err := c.gql.Do(queryComments, vars, &response); err != nil {
			return nil, err
		}

		target := response.Repository.IssueOrPullRequest
		if target == nil {
			return nil, fmt.Errorf("issue or pull request #%d was not found", number)
		}

		comments = append(comments, target.Comments.Nodes...)
		if !target.Comments.PageInfo.HasNextPage {
			break
		}

		endCursor = target.Comments.PageInfo.EndCursor
	}

	return comments, nil
}

func (c *Client) MinimizeComment(id, classifier string) error {
	var response struct {
		MinimizeComment struct {
			MinimizedComment struct {
				IsMinimized bool `json:"isMinimized"`
			} `json:"minimizedComment"`
		} `json:"minimizeComment"`
	}

	if err := c.gql.Do(mutationMinimizeComment, map[string]interface{}{
		"id":     id,
		"reason": classifier,
	}, &response); err != nil {
		return err
	}

	if !response.MinimizeComment.MinimizedComment.IsMinimized {
		return fmt.Errorf("comment %s was not minimized", id)
	}

	return nil
}

func (c *Client) UnminimizeComment(id string) error {
	var response struct {
		UnminimizeComment struct {
			UnminimizedComment struct {
				IsMinimized bool `json:"isMinimized"`
			} `json:"unminimizedComment"`
		} `json:"unminimizeComment"`
	}

	if err := c.gql.Do(mutationUnminimizeComment, map[string]interface{}{
		"id": id,
	}, &response); err != nil {
		return err
	}

	if response.UnminimizeComment.UnminimizedComment.IsMinimized {
		return fmt.Errorf("comment %s is still minimized", id)
	}

	return nil
}

type commentConnection struct {
	Nodes    []Comment `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

const queryComments = `
query($owner: String!, $repo: String!, $number: Int!, $endCursor: String) {
	repository(owner: $owner, name: $repo) {
		issueOrPullRequest(number: $number) {
			... on Issue {
				comments(first: 100, after: $endCursor) {
					nodes {
						id
						author { login }
						bodyText
						isMinimized
						minimizedReason
					}
					pageInfo {
						hasNextPage
						endCursor
					}
				}
			}
			... on PullRequest {
				comments(first: 100, after: $endCursor) {
					nodes {
						id
						author { login }
						bodyText
						isMinimized
						minimizedReason
					}
					pageInfo {
						hasNextPage
						endCursor
					}
				}
			}
		}
	}
}
`

const mutationMinimizeComment = `
mutation($id: ID!, $reason: ReportedContentClassifiers!) {
	minimizeComment(input: { subjectId: $id, classifier: $reason }) {
		minimizedComment {
			isMinimized
		}
	}
}
`

const mutationUnminimizeComment = `
mutation($id: ID!) {
	unminimizeComment(input: { subjectId: $id }) {
		unminimizedComment {
			isMinimized
		}
	}
}
`

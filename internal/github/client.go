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

func (c *Client) FindIssueOrPullRequestComments(owner, repo string, number int) ([]Comment, error) {
	comments, isPullRequest, err := c.findIssueOrPullRequestComments(owner, repo, number)
	if err != nil {
		return nil, err
	}
	if !isPullRequest {
		return comments, nil
	}

	reviewComments, err := c.findPullRequestReviewComments(owner, repo, number)
	if err != nil {
		return nil, err
	}

	return append(comments, reviewComments...), nil
}

func (c *Client) findIssueOrPullRequestComments(owner, repo string, number int) ([]Comment, bool, error) {
	var (
		comments      []Comment
		endCursor     string
		isPullRequest bool
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

		var response commentsQueryResponse

		if err := c.gql.Do(queryComments, vars, &response); err != nil {
			return nil, false, err
		}

		target := response.Repository.IssueOrPullRequest
		if target == nil {
			return nil, false, fmt.Errorf("issue or pull request #%d was not found", number)
		}
		isPullRequest = targetIsPullRequest(target.TypeName)

		for _, comment := range target.Comments.Nodes {
			comments = append(comments, comment.Comment())
		}
		if !target.Comments.PageInfo.HasNextPage {
			break
		}

		endCursor = target.Comments.PageInfo.EndCursor
	}

	return comments, isPullRequest, nil
}

func (c *Client) findPullRequestReviewComments(owner, repo string, number int) ([]Comment, error) {
	var (
		comments     []Comment
		threadCursor string
	)

	for {
		vars := map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": number,
		}
		if threadCursor != "" {
			vars["threadCursor"] = threadCursor
		}

		var response reviewThreadsQueryResponse
		if err := c.gql.Do(queryReviewThreads, vars, &response); err != nil {
			return nil, err
		}

		target := response.Repository.PullRequest
		if target == nil {
			return nil, fmt.Errorf("pull request #%d was not found", number)
		}

		for _, thread := range target.ReviewThreads.Nodes {
			for _, comment := range thread.Comments.Nodes {
				comments = append(comments, comment.Comment())
			}
			if thread.Comments.PageInfo.HasNextPage {
				threadComments, err := c.findReviewThreadComments(thread.ID, thread.Comments.PageInfo.EndCursor)
				if err != nil {
					return nil, err
				}
				comments = append(comments, threadComments...)
			}
		}

		if !target.ReviewThreads.PageInfo.HasNextPage {
			break
		}

		threadCursor = target.ReviewThreads.PageInfo.EndCursor
	}

	return comments, nil
}

func (c *Client) findReviewThreadComments(id, endCursor string) ([]Comment, error) {
	var comments []Comment

	for {
		vars := map[string]interface{}{
			"id": id,
		}
		if endCursor != "" {
			vars["endCursor"] = endCursor
		}

		var response reviewThreadCommentsQueryResponse
		if err := c.gql.Do(queryReviewThreadComments, vars, &response); err != nil {
			return nil, err
		}

		target := response.Node
		if target == nil {
			return nil, fmt.Errorf("review thread %s was not found", id)
		}

		for _, comment := range target.Comments.Nodes {
			comments = append(comments, comment.Comment())
		}

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
	Nodes    []graphqlComment `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

type reviewThread struct {
	ID       string            `json:"id"`
	Comments commentConnection `json:"comments"`
}

type reviewThreadConnection struct {
	Nodes    []reviewThread `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

type commentsQueryResponse struct {
	Repository struct {
		IssueOrPullRequest *struct {
			TypeName string            `json:"__typename"`
			Comments commentConnection `json:"comments"`
		} `json:"issueOrPullRequest"`
	} `json:"repository"`
}

type reviewThreadsQueryResponse struct {
	Repository struct {
		PullRequest *struct {
			ReviewThreads reviewThreadConnection `json:"reviewThreads"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

type reviewThreadCommentsQueryResponse struct {
	Node *struct {
		Comments commentConnection `json:"comments"`
	} `json:"node"`
}

func targetIsPullRequest(typeName string) bool {
	return typeName == "PullRequest"
}

const fragments = `
fragment comment on Comment {
	id
	author {
		__typename
		login
	}
	bodyText
}

fragment minimizable on Minimizable {
	isMinimized
	minimizedReason
}
`

const queryComments = fragments + `
query($owner: String!, $repo: String!, $number: Int!, $endCursor: String) {
	repository(owner: $owner, name: $repo) {
		issueOrPullRequest(number: $number) {
			__typename
			... on Issue {
				comments(first: 100, after: $endCursor) {
					nodes {
						...comment
						...minimizable
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
						...comment
						...minimizable
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

const queryReviewThreads = fragments + `
query($owner: String!, $repo: String!, $number: Int!, $threadCursor: String) {
	repository(owner: $owner, name: $repo) {
		pullRequest(number: $number) {
			reviewThreads(first: 100, after: $threadCursor) {
				nodes {
					id
					comments(first: 100) {
						nodes {
							...comment
							...minimizable
						}
						pageInfo {
							hasNextPage
							endCursor
						}
					}
				}
				pageInfo {
					hasNextPage
					endCursor
				}
			}
		}
	}
}
`

const queryReviewThreadComments = fragments + `
query($id: ID!, $endCursor: String) {
	node(id: $id) {
		... on PullRequestReviewThread {
			comments(first: 100, after: $endCursor) {
				nodes {
					...comment
					...minimizable
				}
				pageInfo {
					hasNextPage
					endCursor
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

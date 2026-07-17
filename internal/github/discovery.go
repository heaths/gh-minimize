package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/cli/cli/v2/git"
	ghrepo "github.com/cli/go-gh/v2/pkg/repository"
)

var (
	resolveCurrentBranch     = func() (string, error) { return (&git.Client{}).CurrentBranch(context.Background()) }
	resolveCurrentRepository = ghrepo.Current
	newDiscoveryClient       = func() (*Client, error) { return New(nil) }
)

func CurrentPullRequestNumber(repoFlag string) (int, error) {
	branch, err := resolveCurrentBranch()
	if err != nil {
		return 0, fmt.Errorf("could not determine current branch: %w", err)
	}

	repo, err := resolveRepositoryForCurrentPullRequest(repoFlag)
	if err != nil {
		return 0, err
	}

	number, err := pullRequestNumberForBranch(branch, repo)
	if err != nil {
		return 0, fmt.Errorf("could not determine pull request for branch %q: %w", branch, err)
	}

	return number, nil
}

func resolveRepositoryForCurrentPullRequest(repoFlag string) (ghrepo.Repository, error) {
	if repoFlag != "" {
		repo, err := ghrepo.Parse(repoFlag)
		if err != nil {
			return ghrepo.Repository{}, fmt.Errorf("invalid repository: %w", err)
		}

		return repo, nil
	}

	repo, err := resolveCurrentRepository()
	if err != nil {
		return ghrepo.Repository{}, fmt.Errorf("could not determine repository; pass --repo OWNER/REPO: %w", err)
	}

	return repo, nil
}

func pullRequestNumberForBranch(branch string, repo ghrepo.Repository) (int, error) {
	client, err := newDiscoveryClient()
	if err != nil {
		return 0, err
	}

	return client.PullRequestNumberForBranch(repo.Owner, repo.Name, branch)
}

func (c *Client) PullRequestNumberForBranch(owner, repo, branch string) (int, error) {
	var response struct {
		Repository struct {
			PullRequests struct {
				Nodes []struct {
					Number int `json:"number"`
				} `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	}

	err := c.gql.Do(queryPullRequestForBranch, map[string]interface{}{
		"owner":       owner,
		"repo":        repo,
		"headRefName": branch,
		"states":      []string{"OPEN"},
	}, &response)
	if err != nil {
		return 0, err
	}

	if len(response.Repository.PullRequests.Nodes) == 0 {
		return 0, errors.New("no pull request found")
	}

	return response.Repository.PullRequests.Nodes[0].Number, nil
}

const queryPullRequestForBranch = `
query PullRequestForBranch($owner: String!, $repo: String!, $headRefName: String!, $states: [PullRequestState!]) {
	repository(owner: $owner, name: $repo) {
		pullRequests(headRefName: $headRefName, states: $states, first: 1, orderBy: { field: CREATED_AT, direction: DESC }) {
			nodes {
				number
			}
		}
	}
}
`

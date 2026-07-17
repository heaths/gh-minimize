package github

import (
	"errors"
	"testing"

	ghrepo "github.com/cli/go-gh/v2/pkg/repository"
	"github.com/stretchr/testify/require"
)

func TestCurrentPullRequestNumber(t *testing.T) {
	t.Run("returns pull request number for current branch", func(t *testing.T) {
		stubPullRequestDiscovery(t, pullRequestDiscoveryStub{
			branch: "feature/test",
			repo:   ghrepo.Repository{Owner: "OWNER", Name: "REPO"},
			number: 42,
		})

		number, err := CurrentPullRequestNumber("")
		require.NoError(t, err)
		require.Equal(t, 42, number)
	})

	t.Run("returns invalid repository error for malformed repo flag", func(t *testing.T) {
		stubPullRequestDiscovery(t, pullRequestDiscoveryStub{
			branch: "feature/test",
		})

		_, err := CurrentPullRequestNumber("@")
		require.ErrorContains(t, err, "invalid repository")
	})

	t.Run("returns branch resolution error", func(t *testing.T) {
		stubPullRequestDiscovery(t, pullRequestDiscoveryStub{
			branchErr: errors.New("not on branch"),
		})

		_, err := CurrentPullRequestNumber("")
		require.ErrorContains(t, err, "could not determine current branch")
	})

	t.Run("returns no pull request found error", func(t *testing.T) {
		stubPullRequestDiscovery(t, pullRequestDiscoveryStub{
			branch: "feature/test",
			repo:   ghrepo.Repository{Owner: "OWNER", Name: "REPO"},
		})

		_, err := CurrentPullRequestNumber("")
		require.ErrorContains(t, err, `could not determine pull request for branch "feature/test"`)
		require.ErrorContains(t, err, "no pull request found")
	})
}

type pullRequestDiscoveryStub struct {
	branch    string
	branchErr error
	repo      ghrepo.Repository
	repoErr   error
	number    int
	queryErr  error
}

func stubPullRequestDiscovery(t *testing.T, stub pullRequestDiscoveryStub) {
	t.Helper()

	oldBranch := resolveCurrentBranch
	oldRepo := resolveCurrentRepository
	oldClient := newDiscoveryClient
	t.Cleanup(func() {
		resolveCurrentBranch = oldBranch
		resolveCurrentRepository = oldRepo
		newDiscoveryClient = oldClient
	})

	resolveCurrentBranch = func() (string, error) {
		if stub.branchErr != nil {
			return "", stub.branchErr
		}
		return stub.branch, nil
	}
	resolveCurrentRepository = func() (ghrepo.Repository, error) {
		if stub.repoErr != nil {
			return ghrepo.Repository{}, stub.repoErr
		}
		return stub.repo, nil
	}
	newDiscoveryClient = func() (*Client, error) {
		return NewWithClient(&fakeGQLClient{
			do: func(query string, _ map[string]interface{}, response interface{}) error {
				if query != queryPullRequestForBranch {
					return errors.New("unexpected query")
				}
				if stub.queryErr != nil {
					return stub.queryErr
				}

				resp := response.(*struct {
					Repository struct {
						PullRequests struct {
							Nodes []struct {
								Number int `json:"number"`
							} `json:"nodes"`
						} `json:"pullRequests"`
					} `json:"repository"`
				})
				if stub.number > 0 {
					resp.Repository.PullRequests.Nodes = []struct {
						Number int `json:"number"`
					}{
						{Number: stub.number},
					}
				}

				return nil
			},
		}), nil
	}
}

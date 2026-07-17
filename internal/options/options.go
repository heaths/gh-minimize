package options

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/repository"
)

func ResolveRepository(repoFlag string) (repository.Repository, error) {
	if repoFlag != "" {
		repo, err := repository.Parse(repoFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid repository: %w", err)
		}

		return repo, nil
	}

	repo, err := gh.CurrentRepository()
	if err != nil {
		return nil, fmt.Errorf("could not determine repository; pass --repo OWNER/REPO: %w", err)
	}

	return repo, nil
}

func ResolveIssueOrPullRequestNumber(args []string) (int, error) {
	switch len(args) {
	case 1:
		return ParseNumber(args[0])
	default:
		return 0, errors.New("expected exactly one issue or pull request number argument")
	}
}

func ParseNumber(number string) (int, error) {
	value := strings.TrimPrefix(number, "#")
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid issue or pull request number %q", number)
	}

	return int(parsed), nil
}

func RepoArgument(repo repository.Repository) string {
	if repo.Host() == "" || repo.Host() == "github.com" {
		return fmt.Sprintf("%s/%s", repo.Owner(), repo.Name())
	}

	return fmt.Sprintf("%s/%s/%s", repo.Host(), repo.Owner(), repo.Name())
}

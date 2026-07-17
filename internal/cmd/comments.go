package cmd

import (
	"fmt"
	"regexp"

	ghclient "github.com/heaths/gh-minimize/internal/github"
	"github.com/heaths/gh-minimize/internal/options"
)

func ensureClient(client commentService) (commentService, error) {
	if client != nil {
		return client, nil
	}

	resolved, err := ghclient.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	return resolved, nil
}

func loadFilteredComments(client commentService, repoFlag string, args []string, authors []string, bodyGrep string) ([]ghclient.Comment, error) {
	repo, err := options.ResolveRepository(repoFlag)
	if err != nil {
		return nil, err
	}

	targetNumber, err := options.ResolveIssueOrPullRequestNumber(args)
	if err != nil {
		return nil, err
	}

	bodyRegex, err := compileBodyRegex(bodyGrep)
	if err != nil {
		return nil, err
	}

	comments, err := client.FindIssueOrPullRequestComments(repo.Owner(), repo.Name(), targetNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to find comments: %w", err)
	}

	return filterComments(comments, authors, bodyRegex), nil
}

func compileBodyRegex(bodyGrep string) (*regexp.Regexp, error) {
	if bodyGrep == "" {
		return nil, nil
	}

	bodyRegex, err := regexp.Compile(bodyGrep)
	if err != nil {
		return nil, fmt.Errorf("invalid --body-grep regex: %w", err)
	}

	return bodyRegex, nil
}

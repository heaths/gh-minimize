package cmd

import (
	"fmt"
	"io"
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

func loadFilteredComments(client commentService, output io.Writer, repoFlag string, args []string, authors []string, grep string, invertGrep bool) ([]ghclient.Comment, error) {
	repo, err := options.ResolveRepository(repoFlag)
	if err != nil {
		return nil, err
	}

	targetNumber, err := options.ResolveIssueOrPullRequestNumber(args, repoFlag)
	if err != nil {
		return nil, err
	}

	grepRegex, err := compileGrepRegex(grep)
	if err != nil {
		return nil, err
	}

	indicator := startProgress(output)
	defer stopProgress(indicator)

	comments, err := client.FindIssueOrPullRequestComments(repo.Owner, repo.Name, targetNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to find comments: %w", err)
	}

	return filterComments(comments, authors, grepRegex, invertGrep), nil
}

func compileGrepRegex(grep string) (*regexp.Regexp, error) {
	if grep == "" {
		return nil, nil
	}

	grepRegex, err := regexp.Compile(grep)
	if err != nil {
		return nil, fmt.Errorf("invalid --grep regex: %w", err)
	}

	return grepRegex, nil
}

package cmd

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/MakeNowJust/heredoc"
	ghclient "github.com/heaths/gh-minimize/internal/github"
	"github.com/heaths/gh-minimize/internal/options"
	"github.com/spf13/cobra"
)

type commentService interface {
	FindIssueOrPullRequestComments(owner, repo string, number int) ([]ghclient.Comment, error)
	MinimizeComment(id, classifier string) error
	UnminimizeComment(id string) error
}

type rootOptions struct {
	id       string
	author   string
	bodyGrep string
	reason   string
	undo     bool
	repo     string

	stdout io.Writer
	stderr io.Writer

	client commentService
}

func New() *cobra.Command {
	opts := &rootOptions{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	cmd := &cobra.Command{
		Use:   "minimize [issue-or-pr-number]",
		Short: "Minimize or unminimize issue and pull request comments",
		Long: heredoc.Doc(`
			Minimize or unminimize issue and pull request comments by node ID
			or by searching comment author and body text.
		`),
		Example: heredoc.Doc(`
			$ gh minimize --id MDEyOklzc3VlQ29tbWVudDE= --reason off-topic
			$ gh minimize --id MDEyOklzc3VlQ29tbWVudDE= --undo
			$ gh minimize 123 --author octocat --body-grep 'obsolete.*context' --reason outdated
			$ gh minimize 123 --author octocat --body-grep 'obsolete.*context' --undo
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(opts, args)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.id, "id", "", "Comment node ID")
	flags.StringVar(&opts.author, "author", "", "Comment author login filter")
	flags.StringVar(&opts.bodyGrep, "body-grep", "", "Go regular expression to filter comment body text")
	flags.StringVar(&opts.reason, "reason", "", fmt.Sprintf("Minimization reason (%s)", strings.Join(ghclient.AllowedReasons(), ", ")))
	flags.BoolVar(&opts.undo, "undo", false, "Unminimize comments")
	flags.StringVarP(&opts.repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	_ = cmd.RegisterFlagCompletionFunc("reason", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return ghclient.AllowedReasons(), cobra.ShellCompDirectiveNoFileComp
	})
	cmd.MarkFlagsMutuallyExclusive("id", "author")
	cmd.MarkFlagsMutuallyExclusive("id", "body-grep")
	cmd.MarkFlagsMutuallyExclusive("undo", "reason")

	return cmd
}

func run(opts *rootOptions, args []string) error {
	if err := validateFlags(opts, args); err != nil {
		return err
	}

	if opts.client == nil {
		client, err := ghclient.New(nil)
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		opts.client = client
	}

	if opts.id != "" {
		return applyAction(opts, []string{opts.id})
	}

	repo, err := options.ResolveRepository(opts.repo)
	if err != nil {
		return err
	}

	targetNumber, err := options.ResolveIssueOrPullRequestNumber(args)
	if err != nil {
		return err
	}

	var bodyRegex *regexp.Regexp
	if opts.bodyGrep != "" {
		bodyRegex, err = regexp.Compile(opts.bodyGrep)
		if err != nil {
			return fmt.Errorf("invalid --body-grep regex: %w", err)
		}
	}

	comments, err := opts.client.FindIssueOrPullRequestComments(repo.Owner(), repo.Name(), targetNumber)
	if err != nil {
		return fmt.Errorf("failed to find comments: %w", err)
	}

	ids := filterCommentIDs(comments, opts.author, bodyRegex, opts.undo)
	return applyAction(opts, ids)
}

func validateFlags(opts *rootOptions, args []string) error {
	if opts.undo && opts.reason != "" {
		return fmt.Errorf("--undo cannot be used with --reason")
	}
	if !opts.undo && opts.reason == "" {
		return fmt.Errorf("either --reason or --undo must be provided")
	}
	if opts.reason != "" {
		if _, err := ghclient.ParseReason(opts.reason); err != nil {
			return err
		}
	}
	if opts.id != "" {
		if len(args) > 0 {
			return fmt.Errorf("--id cannot be used with an issue or pull request number")
		}
		if opts.author != "" || opts.bodyGrep != "" {
			return fmt.Errorf("--id cannot be used with --author or --body-grep")
		}
		return nil
	}
	if opts.author == "" && opts.bodyGrep == "" {
		return fmt.Errorf("at least one of --author or --body-grep is required when --id is not provided")
	}
	if len(args) != 1 {
		return fmt.Errorf("exactly one issue or pull request number argument is required when --id is not provided")
	}

	return nil
}

func filterCommentIDs(comments []ghclient.Comment, author string, bodyRegex *regexp.Regexp, undo bool) []string {
	ids := make([]string, 0, len(comments))

	for _, comment := range comments {
		if author != "" && !strings.EqualFold(comment.Author.Login, author) {
			continue
		}
		if bodyRegex != nil && !bodyRegex.MatchString(comment.Body) {
			continue
		}
		if undo && !comment.IsMinimized {
			continue
		}
		if !undo && comment.IsMinimized {
			continue
		}

		ids = append(ids, comment.ID)
	}

	return ids
}

func applyAction(opts *rootOptions, ids []string) error {
	if len(ids) == 0 {
		_, _ = fmt.Fprintln(opts.stdout, "No matching comments found.")
		return nil
	}

	var reason string
	var err error
	if !opts.undo {
		reason, err = ghclient.ParseReason(opts.reason)
		if err != nil {
			return err
		}
	}

	updated := 0
	for _, id := range ids {
		if opts.undo {
			err = opts.client.UnminimizeComment(id)
		} else {
			err = opts.client.MinimizeComment(id, reason)
		}
		if err != nil {
			return fmt.Errorf("failed to update comment %s: %w", id, err)
		}
		updated++
	}

	action := "Minimized"
	if opts.undo {
		action = "Unminimized"
	}
	_, _ = fmt.Fprintf(opts.stdout, "%s %d comment(s).\n", action, updated)
	return nil
}

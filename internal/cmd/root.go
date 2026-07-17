package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/pkg/iostreams"
	ghclient "github.com/heaths/gh-minimize/internal/github"
	"github.com/heaths/gh-minimize/internal/options"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commentService interface {
	FindIssueOrPullRequestComments(owner, repo string, number int) ([]ghclient.Comment, error)
	MinimizeComment(id, classifier string) error
	UnminimizeComment(id string) error
}

type commonOptions struct {
	io     *iostreams.IOStreams
	global *globalOptions
	client commentService
}

type filterOptions struct {
	authors  []string
	bodyGrep string
}

type rootOptions struct {
	commonOptions
	filterOptions
	id     string
	reason string
	undo   bool
}

type listOptions struct {
	commonOptions
	filterOptions
	jsonFields   string
	jqExpression string
	tmpl         string
}

type globalOptions struct {
	repo string
}

var executableName = func() string {
	if len(os.Args) == 0 {
		return "gh-minimize"
	}

	return filepath.Base(os.Args[0])
}

func New() *cobra.Command {
	return NewWithIO(iostreams.System())
}

func NewWithIO(streams *iostreams.IOStreams) *cobra.Command {
	displayName := commandDisplayName()
	globalOpts := &globalOptions{}
	opts := &rootOptions{
		commonOptions: commonOptions{
			io:     streams,
			global: globalOpts,
		},
	}
	listOpts := &listOptions{
		commonOptions: commonOptions{
			io:     streams,
			global: globalOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [issue-or-pr-number]", displayName),
		Short: "Minimize or unminimize issue and pull request comments",
		Long: heredoc.Doc(`
			Minimize or unminimize issue and pull request comments by node ID
			or by searching comment authors and body text.
		`),
		Example: heredoc.Docf(`
			$ %[1]s --id MDEyOklzc3VlQ29tbWVudDE= --reason off-topic
			$ %[1]s --id MDEyOklzc3VlQ29tbWVudDE= --undo
			$ %[1]s 123 --author octocat --body-grep 'obsolete.*context' --reason outdated
			$ %[1]s 123 --author octocat --author hubot --reason resolved
			$ %[1]s 123 --author octocat --body-grep 'obsolete.*context' --undo
			$ %[1]s list 123
		`, displayName),
		Args: positionalIssueOrPullRequestArgs(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(opts, args)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(streams.Out)
	cmd.SetErr(streams.ErrOut)

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.StringVarP(&globalOpts.repo, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")

	flags := cmd.Flags()
	flags.StringVar(&opts.id, "id", "", "Comment node ID")
	addFilterFlags(flags, &opts.filterOptions)
	flags.StringVar(&opts.reason, "reason", "", fmt.Sprintf("Minimization reason (%s)", strings.Join(ghclient.AllowedReasons(), ", ")))
	flags.BoolVar(&opts.undo, "undo", false, "Unminimize comments")
	_ = cmd.RegisterFlagCompletionFunc("reason", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return ghclient.AllowedReasons(), cobra.ShellCompDirectiveNoFileComp
	})
	cmd.MarkFlagsMutuallyExclusive("id", "author")
	cmd.MarkFlagsMutuallyExclusive("id", "body-grep")
	cmd.MarkFlagsMutuallyExclusive("undo", "reason")

	listCmd := &cobra.Command{
		Use:   "list <issue-or-pr-number>",
		Short: "List issue or review comments to find IDs",
		Long:  "List issue or review comments so you can find comment IDs.",
		Args:  positionalIssueOrPullRequestArgs(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(listOpts, args)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	listFlags := listCmd.Flags()
	addFilterFlags(listFlags, &listOpts.filterOptions)
	listFlags.StringVar(&listOpts.jsonFields, "json", "", fmt.Sprintf("Output JSON with the specified fields (%s)", strings.Join(ghclient.CommentFields(), ",")))
	listFlags.StringVar(&listOpts.jqExpression, "jq", "", "Filter JSON output using a jq expression")
	listFlags.StringVar(&listOpts.tmpl, "template", "", "Format JSON output using a Go template")
	listCmd.MarkFlagsMutuallyExclusive("jq", "template")
	cmd.AddCommand(listCmd)

	return cmd
}

func addFilterFlags(flags *pflag.FlagSet, opts *filterOptions) {
	flags.StringArrayVar(&opts.authors, "author", nil, "Comment author login filter; repeat to match any specified login")
	flags.StringVar(&opts.bodyGrep, "body-grep", "", "Go regular expression to filter comment body text")
}

func positionalIssueOrPullRequestArgs(required bool) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if required {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}
		} else if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}

		if len(args) == 0 {
			return nil
		}

		_, err := options.ResolveIssueOrPullRequestNumber(args)
		return err
	}
}

func commandDisplayName() string {
	if _, ok := os.LookupEnv("GH_EXTENSION"); ok {
		return "gh minimize"
	}

	return executableName()
}

func run(opts *rootOptions, args []string) error {
	if err := validateFlags(opts, args); err != nil {
		return err
	}

	client, err := ensureClient(opts.client)
	if err != nil {
		return err
	}
	opts.client = client

	if opts.id != "" {
		return applyAction(opts, []string{opts.id})
	}

	comments, err := loadFilteredComments(opts.client, opts.repoFlag(), args, opts.authors, opts.bodyGrep)
	if err != nil {
		return err
	}

	ids := filterCommentIDs(comments, opts.undo)
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
		if len(opts.authors) > 0 || opts.bodyGrep != "" {
			return fmt.Errorf("--id cannot be used with --author or --body-grep")
		}
		return nil
	}
	if len(opts.authors) == 0 && opts.bodyGrep == "" {
		return fmt.Errorf("at least one of --author or --body-grep is required when --id is not provided")
	}
	if len(args) != 1 {
		return fmt.Errorf("exactly one issue or pull request number argument is required when --id is not provided")
	}

	return nil
}

func (opts commonOptions) repoFlag() string {
	if opts.global != nil {
		return opts.global.repo
	}

	return ""
}

func filterCommentIDs(comments []ghclient.Comment, undo bool) []string {
	ids := make([]string, 0, len(comments))

	for _, comment := range comments {
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

func matchesAuthor(login string, authors []string) bool {
	for _, author := range authors {
		if strings.EqualFold(login, author) {
			return true
		}
	}

	return false
}

func applyAction(opts *rootOptions, ids []string) error {
	if len(ids) == 0 {
		_, _ = fmt.Fprintln(opts.io.Out, "No matching comments found.")
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
	_, _ = fmt.Fprintf(opts.io.Out, "%s %d comment(s).\n", action, updated)
	return nil
}

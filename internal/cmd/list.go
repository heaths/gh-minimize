package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"

	ghjq "github.com/cli/go-gh/v2/pkg/jq"
	"github.com/cli/go-gh/v2/pkg/jsonpretty"
	ghtemplate "github.com/cli/go-gh/v2/pkg/template"
	"github.com/cli/go-gh/v2/pkg/term"
	ghclient "github.com/heaths/gh-minimize/internal/github"
)

func runList(opts *listOptions, args []string) error {
	client, err := ensureClient(opts.client)
	if err != nil {
		return err
	}
	opts.client = client

	comments, err := loadFilteredComments(opts.client, opts.repoFlag(), args, opts.authors, opts.bodyGrep)
	if err != nil {
		return err
	}
	return writeCommentOutput(opts, comments)
}

func writeCommentOutput(opts *listOptions, comments []ghclient.Comment) error {
	data, err := marshalCommentOutput(opts, comments)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(data)
	switch {
	case opts.tmpl != "":
		tmpl := ghtemplate.New(opts.term.Out(), terminalWidth(opts.term), opts.term.IsColorEnabled())
		if err := tmpl.Parse(opts.tmpl); err != nil {
			return err
		}
		if err := tmpl.Execute(reader); err != nil {
			return err
		}
		return tmpl.Flush()
	case opts.jqExpression != "":
		return ghjq.Evaluate(reader, opts.term.Out(), opts.jqExpression)
	default:
		return writeJSONOutput(opts.term, reader)
	}
}

func marshalCommentOutput(opts *listOptions, comments []ghclient.Comment) ([]byte, error) {
	if opts.jsonFields == "" {
		return marshalJSON(comments)
	}

	fields, err := ghclient.ParseCommentFields(opts.jsonFields)
	if err != nil {
		return nil, err
	}

	data, err := ghclient.ExportComments(comments, fields)
	if err != nil {
		return nil, err
	}

	return marshalJSON(data)
}

func filterComments(comments []ghclient.Comment, authors []string, bodyRegex *regexp.Regexp) []ghclient.Comment {
	filtered := make([]ghclient.Comment, 0, len(comments))

	for _, comment := range comments {
		if len(authors) > 0 && !matchesAuthor(comment.Author, authors) {
			continue
		}
		if bodyRegex != nil && !bodyRegex.MatchString(comment.Body) {
			continue
		}

		filtered = append(filtered, comment)
	}

	return filtered
}

func marshalJSON(v interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func writeJSONOutput(terminal term.Term, input io.Reader) error {
	if err := prettyPrintJSONOutput(terminal, input); err == nil {
		return nil
	}

	_, err := io.Copy(terminal.Out(), input)
	return err
}

func prettyPrintJSONOutput(terminal term.Term, input io.Reader) error {
	if !terminal.IsTerminalOutput() {
		return ioCopyUnsupported{}
	}

	return jsonpretty.Format(terminal.Out(), input, "  ", terminal.IsColorEnabled())
}

type ioCopyUnsupported struct{}

func (ioCopyUnsupported) Error() string { return "pretty JSON output is not supported" }

func terminalWidth(terminal term.Term) int {
	width, _, err := terminal.Size()
	if err == nil && width > 0 {
		return width
	}

	return 80
}

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"

	ghjq "github.com/cli/go-gh/pkg/jq"
	"github.com/cli/go-gh/pkg/jsonpretty"
	ghtemplate "github.com/cli/go-gh/pkg/template"
	ghterm "github.com/cli/go-gh/pkg/term"
	ghclient "github.com/heaths/gh-minimize/internal/github"
)

var detectPrettyJSONSupport = func(w io.Writer) (enabled bool, colorize bool) {
	if w == os.Stdout {
		terminal := ghterm.FromEnv()
		return terminal.IsTerminalOutput(), terminal.IsColorEnabled()
	}

	file, ok := w.(*os.File)
	if !ok || !ghterm.IsTerminal(file) {
		return false, false
	}

	return true, false
}

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
		tmpl := ghtemplate.New(opts.stdout, 120, false)
		if err := tmpl.Parse(opts.tmpl); err != nil {
			return err
		}
		if err := tmpl.Execute(reader); err != nil {
			return err
		}
		return tmpl.Flush()
	case opts.jqExpression != "":
		return ghjq.Evaluate(reader, opts.stdout, opts.jqExpression)
	default:
		return writeJSONOutput(opts.stdout, reader)
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

func writeJSONOutput(w io.Writer, input io.Reader) error {
	if err := prettyPrintJSONOutput(w, input); err == nil {
		return nil
	}

	_, err := io.Copy(w, input)
	return err
}

func prettyPrintJSONOutput(w io.Writer, input io.Reader) error {
	enabled, colorize := detectPrettyJSONSupport(w)
	if !enabled {
		return fmt.Errorf("pretty JSON output is not supported")
	}

	return jsonpretty.Format(w, input, "  ", colorize)
}

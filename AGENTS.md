# AGENTS

## Purpose
- `gh minimize` minimizes or unminimizes issue/PR comments
- target by `--id` or by comment search in a specific issue/PR

## Stack and conventions
- language Go
- CLI framework `spf13/cobra`
- GitHub integration prefer `go-gh` and GitHub GraphQL
- follow idiomatic Go style and stdlib-first design
- keep changes small explicit typed and testable

## Command rules
- forms
  - `gh minimize --id <node-id> --reason <reason>`
  - `gh minimize --id <node-id> --undo`
  - `gh minimize <number> --author <login> [--body-grep <regex>] --reason <reason>`
  - `gh minimize <number> --author <login> [--body-grep <regex>] --undo`
  - `gh minimize <number> --body-grep <regex> --reason <reason>`
  - `gh minimize <number> --body-grep <regex> --undo`
- without `--id` exactly one positional issue/PR number is required
- `--id` is mutually exclusive with `--author` and `--body-grep`
- `--undo` is mutually exclusive with `--reason`
- one of `--reason` or `--undo` is required
- in non-`--id` mode at least one of `--author` or `--body-grep` is required
- `-R/--repo` supports `[HOST/]OWNER/REPO` else current repo

## Reason values
- `abuse`
- `duplicate`
- `low-quality`
- `off-topic`
- `outdated`
- `resolved`
- `spam`

## Architecture
- `main.go` CLI entrypoint executes cobra root command
- `internal/cmd/root.go` command wiring flag validation filtering action flow
- `internal/options/options.go` repo parsing and issue/PR number parsing
- `internal/github/classifier.go` reason mapping to GraphQL enum
- `internal/github/client.go` GraphQL query and mutations
  - query comments via `repository -> issueOrPullRequest -> comments`
  - mutate via `minimizeComment` and `unminimizeComment`

## Testing and tooling
- format with `gofmt`
- manage deps with `go mod tidy`
- run tests with `go test ./...`
- add focused unit tests beside changed behavior
  - `internal/cmd/root_test.go`
  - `internal/options/options_test.go`
  - `internal/github/*_test.go`

## Output behavior
- print summary only
  - `No matching comments found.`
  - `Minimized N comment(s).`
  - `Unminimized N comment(s).`

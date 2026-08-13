# gh minimize

[![releases](https://img.shields.io/github/v/release/heaths/gh-minimize.svg?logo=github)](https://github.com/heaths/gh-minimize/releases/latest)
[![ci](https://github.com/heaths/gh-minimize/actions/workflows/ci.yml/badge.svg?event=push)](https://github.com/heaths/gh-minimize/actions/workflows/ci.yml)

GitHub CLI extension to minimize (hide) issue and pull request comments
with a reason such as "off-topic", "resolved", or "spam".

## Install

Make sure you have version 2.0 or
[newer](https://github.com/cli/cli/releases/latest) of the GitHub CLI installed.

```bash
gh extension install heaths/gh-minimize
```

## Usage

### List comments

List issue or review comments so you can find comment IDs:

```bash
gh minimize list 123
gh minimize list 123 --author octocat --author hubot --grep 'obsolete.*context'
gh minimize list 123 --json id,author,isMinimized
gh minimize list 123 --jq '.[].author'
gh minimize list 123 --jq '[.[] | select(.authorType == "bot")]'
gh minimize list 123 --template '{{range .}}{{printf "%s\t%t\n" .author .isMinimized}}{{end}}'
```

Use `-R` / `--repo` to target another repository in `[HOST/]OWNER/REPO` format.

### Minimize or unminimize comments

Minimize or unminimize a comment directly by ID:

```bash
gh minimize --id MDEyOklzc3VlQ29tbWVudDE= --reason off-topic
gh minimize --id MDEyOklzc3VlQ29tbWVudDE= --undo
```

Filter comments in an issue or pull request by author and/or comment text
(`--grep` supports basic regular expressions only; add `--invert-grep` to
match comments that do *not* match `--grep`):

```bash
gh minimize 123 --author octocat --grep 'obsolete.*context' --reason outdated
gh minimize 123 --author octocat --grep 'obsolete.*context' --undo
```

Valid `--reason` values:

* `abuse`
* `duplicate`
* `low-quality`
* `off-topic`
* `outdated`
* `resolved`
* `spam`

### GitHub Actions

You can use this extension in an `issue_comment` workflow, which GitHub uses
for both issues and pull requests, with only the permissions needed to
minimize comments:

```yaml
on:
  issue_comment:
    types:
    - created
    - edited

permissions:
  pull-requests: write

jobs:
  minimize:
    if: ${{ github.event.issue.pull_request }}
    runs-on: ubuntu-latest
    steps:
    - run: |
        gh ext install heaths/gh-minimize
        gh minimize ${{ github.event.pull_request.number }} --author github-actions --grep 'new changes have been pushed to this pull request' --reason outdated
      env:
        GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Environment variables

While processing comments, `gh minimize` displays a spinner on
standard error when standard error is a TTY. Set `GH_SPINNER_DISABLED=1` or
`GH_SPINNER_DISABLED=true` to disable it.

Run `gh help environment` for more details about this and other environment
variables such as `GH_REPO` and `NO_COLOR`.

## License

Licensed under the [MIT](LICENSE.txt) license.

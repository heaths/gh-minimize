# Gh-Minimize

[![releases](https://img.shields.io/github/v/release/heaths/gh-minimize.svg?logo=github)](https://github.com/heaths/gh-minimize/releases/latest)
[![ci](https://github.com/heaths/gh-minimize/actions/workflows/ci.yml/badge.svg?event=push)](https://github.com/heaths/gh-minimize/actions/workflows/ci.yml)

GitHub CLI extension to minimize or unminimize issue and pull request comments.

## Install

Make sure you have version 2.0 or
[newer](https://github.com/cli/cli/releases/latest) of the GitHub CLI installed.

```bash
gh extension install heaths/gh-minimize
```

## Usage

### Minimize or unminimize comments

Minimize or unminimize a comment directly by node ID:

```bash
gh minimize --id MDEyOklzc3VlQ29tbWVudDE= --reason off-topic
gh minimize --id MDEyOklzc3VlQ29tbWVudDE= --undo
```

Filter comments in an issue or pull request by author and/or body regex:

```bash
gh minimize 123 --author octocat --body-grep 'obsolete.*context' --reason outdated
gh minimize 123 --author octocat --body-grep 'obsolete.*context' --undo
```

Valid `--reason` values:

* `abuse`
* `duplicate`
* `low-quality`
* `off-topic`
* `outdated`
* `resolved`
* `spam`

### List comments

List issue or review comments so you can find comment IDs:

```bash
gh minimize list 123
gh minimize list 123 --author octocat --author hubot --body-grep 'obsolete.*context'
gh minimize list 123 --json id,author,isMinimized
gh minimize list 123 --jq '.[].author'
gh minimize list 123 --template '{{range .}}{{printf "%s\t%t\n" .author .isMinimized}}{{end}}'
```

Use `-R` / `--repo` to target another repository in `[HOST/]OWNER/REPO` format.

## License

Licensed under the [MIT](LICENSE.txt) license.

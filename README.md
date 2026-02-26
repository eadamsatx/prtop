# prtop

A live-updating terminal UI for GitHub PR check statuses, similar to `top`.

## Prerequisites

- Go 1.21+
- [`gh` CLI](https://cli.github.com/) installed and authenticated

## Build

```sh
make build
```

This produces a `./prtop` binary.

## Install

```sh
make install
```

Or install directly:

```sh
go install github.com/eadamsatx/prtop@latest
```

## Usage

```sh
# Pick from your recent open PRs
prtop

# Using a PR URL
prtop https://github.com/owner/repo/pull/123

# Using owner/repo and PR number
prtop owner/repo 123

# With custom refresh interval (default: 5s)
prtop --interval 10 owner/repo 123
```

When run with no arguments, `prtop` shows your 5 most recent open PRs (across all repos) and lets you pick one to view.

## Note: API Rate Limits

prtop polls the GitHub API via `gh` at the configured interval (default 5 seconds), consuming approximately 720 requests/hour. GitHub's authenticated rate limit is 5,000 requests/hour, so this is fine for normal use. However, running multiple instances simultaneously or setting a very low `--interval` could consume your rate limit more quickly. You can increase the interval to reduce API usage:

```sh
prtop --interval 30 owner/repo 123  # ~120 requests/hour
```

## Keybindings

| Key         | Action                        |
|-------------|-------------------------------|
| `q`         | Quit                          |
| `r`         | Force refresh                 |
| `up` / `k`  | Move selection up             |
| `down` / `j`| Move selection down           |
| `enter`     | Open selected check in browser|

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is prtop

A live-updating terminal UI for monitoring GitHub PR check statuses, built with Go using Bubble Tea and Lip Gloss. Requires the `gh` CLI to be installed and authenticated.

## Build & Test Commands

```bash
make build          # compile to ./prtop
make run ARGS="..." # build and run with args
make install        # go install .
make fmt            # go fmt ./...
make lint           # go vet ./...
go test -v -count=1 ./...           # run all tests
go test -v -run TestFilteredChecks  # run a single test
```

## Architecture

Three files form the core, each with a corresponding `_test.go`:

- **main.go** — Entry point, flag parsing, PR URL parsing, `gh` CLI availability check, Bubble Tea program startup
- **gh.go** — GitHub data layer. Defines `Check`, `PRData`, `CheckStatus`, `PRSummary` types. Shells out to `gh` CLI for all GitHub API calls (`gh pr view`, `gh search prs`). Normalizes heterogeneous check statuses (CheckRun vs StatusContext) into four states: `Running`, `Fail`, `Pass`, `Skipped`. `exec.Command` is injectable via `var execCommand` for test mocking.
- **ui.go** — Bubble Tea model with two view modes: `modeSelecting` (PR picker list) and `modeViewing` (check details table). Handles keyboard navigation, auto-refresh on a configurable tick interval, status filtering (skipped hidden by default), and viewport scrolling. Uses Lip Gloss styles for colored/styled terminal output.

## Key Patterns

- **exec.Command injection**: `gh.go` uses `var execCommand = exec.Command` so tests can substitute a mock process via `TestHelperProcess`.
- **Status normalization**: GitHub returns different status fields depending on whether a check is a CheckRun or StatusContext. `normalizeStatus()` maps all variants (SUCCESS, FAILURE, IN_PROGRESS, SKIPPED, NEUTRAL, etc.) to the four `CheckStatus` iota values. Checks are sorted by status priority (Running < Fail < Pass < Skipped), then alphabetically.
- **Filtered vs unfiltered checks**: The summary line always counts from the unfiltered `m.prData.Checks` for accurate totals. Navigation and rendering use `m.filteredChecks()` which respects `hideSkipped`.

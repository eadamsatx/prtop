package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func parsePRURL(url string) (repo string, prNumber string, ok bool) {
	// https://github.com/owner/repo/pull/123
	url = strings.TrimRight(url, "/")
	parts := strings.Split(url, "/")
	// Expected: ["https:", "", "github.com", "owner", "repo", "pull", "123"]
	if len(parts) < 7 {
		return "", "", false
	}
	if parts[2] != "github.com" || parts[5] != "pull" {
		return "", "", false
	}
	repo = parts[3] + "/" + parts[4]
	prNumber = parts[6]
	if prNumber == "" {
		return "", "", false
	}
	return repo, prNumber, true
}

func main() {
	interval := flag.Int("interval", 5, "Refresh interval in seconds")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: prtop [--interval N] [PR-URL | owner/repo PR-number]\n\n")
		fmt.Fprintf(os.Stderr, "Live-updating terminal UI for GitHub PR check statuses.\n\n")
		fmt.Fprintf(os.Stderr, "When run with no arguments, shows your 5 most recent open PRs to select from.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  prtop                                            # pick from recent PRs\n")
		fmt.Fprintf(os.Stderr, "  prtop https://github.com/owner/repo/pull/123\n")
		fmt.Fprintf(os.Stderr, "  prtop owner/repo 123\n")
		fmt.Fprintf(os.Stderr, "  prtop --interval 10 owner/repo 123\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) > 2 {
		flag.Usage()
		os.Exit(1)
	}

	// Check gh is available
	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: 'gh' CLI not found on PATH.\n")
		fmt.Fprintf(os.Stderr, "Install it from https://cli.github.com/\n")
		os.Exit(1)
	}

	var m model
	dur := time.Duration(*interval) * time.Second
	switch len(args) {
	case 0:
		m = newSelectModel(dur)
	case 1:
		repo, prNumber, ok := parsePRURL(args[0])
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid PR URL: %s\n", args[0])
			fmt.Fprintf(os.Stderr, "Expected format: https://github.com/owner/repo/pull/123\n")
			os.Exit(1)
		}
		m = newModel(repo, prNumber, dur)
	default:
		m = newModel(args[0], args[1], dur)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

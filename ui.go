package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	stylePass    = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	styleFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleSkipped = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleUnder   = lipgloss.NewStyle().Underline(true)
	styleReverse = lipgloss.NewStyle().Reverse(true)

	styleHeader     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	styleRepo       = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	stylePRNumber   = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	styleTitle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleUpdatedAt  = lipgloss.NewStyle().Faint(true)
	styleSelected   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	styleSelectedBg = lipgloss.NewStyle().Background(lipgloss.Color("236"))
)

// View modes
type viewMode int

const (
	modeSelecting viewMode = iota
	modeViewing
)

// Messages
type prDataMsg struct {
	data *PRData
	err  error
}

type prListMsg struct {
	prs []PRSummary
	err error
}

type tickMsg time.Time

// Model
type model struct {
	mode     viewMode
	repo     string
	prNumber string
	interval time.Duration
	prData   *PRData
	err      error
	selected int
	width    int
	height   int
	// Selection mode fields
	prs     []PRSummary
	loading bool
}

func newModel(repo, prNumber string, interval time.Duration) model {
	return model{
		mode:     modeViewing,
		repo:     repo,
		prNumber: prNumber,
		interval: interval,
	}
}

func newSelectModel(interval time.Duration) model {
	return model{
		mode:     modeSelecting,
		interval: interval,
		loading:  true,
	}
}

func fetchPRListCmd() tea.Cmd {
	return func() tea.Msg {
		prs, err := fetchRecentPRs()
		return prListMsg{prs: prs, err: err}
	}
}

func (m model) Init() tea.Cmd {
	if m.mode == modeSelecting {
		return fetchPRListCmd()
	}
	return tea.Batch(m.fetchCmd(), m.tickCmd())
}

func (m model) fetchCmd() tea.Cmd {
	repo := m.repo
	prNumber := m.prNumber
	return func() tea.Msg {
		data, err := fetchPRData(repo, prNumber)
		return prDataMsg{data: data, err: err}
	}
}

func (m model) tickCmd() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp:
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown:
			if m.mode == modeSelecting {
				if len(m.prs) > 0 && m.selected < len(m.prs)-1 {
					m.selected++
				}
			} else if m.prData != nil && m.selected < len(m.prData.Checks)-1 {
				m.selected++
			}
		case tea.KeyEnter:
			if m.mode == modeSelecting {
				if len(m.prs) > 0 {
					pr := m.prs[m.selected]
					m.repo = pr.Repo
					m.prNumber = fmt.Sprintf("%d", pr.Number)
					m.mode = modeViewing
					m.selected = 0
					m.prData = nil
					m.err = nil
					return m, tea.Batch(m.fetchCmd(), m.tickCmd())
				}
			} else if m.prData != nil && len(m.prData.Checks) > 0 {
				check := m.prData.Checks[m.selected]
				if check.DetailsURL != "" {
					openBrowser(check.DetailsURL)
				}
			}
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "q":
				return m, tea.Quit
			case "r":
				if m.mode == modeSelecting {
					m.loading = true
					return m, fetchPRListCmd()
				}
				return m, m.fetchCmd()
			case "k":
				if m.selected > 0 {
					m.selected--
				}
			case "j":
				if m.mode == modeSelecting {
					if len(m.prs) > 0 && m.selected < len(m.prs)-1 {
						m.selected++
					}
				} else if m.prData != nil && m.selected < len(m.prData.Checks)-1 {
					m.selected++
				}
			}
		}

	case prListMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.prs = msg.prs
			m.err = nil
			m.selected = 0
		}

	case prDataMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.prData = msg.data
			m.err = nil
			// Clamp selection
			if len(m.prData.Checks) > 0 {
				if m.selected >= len(m.prData.Checks) {
					m.selected = len(m.prData.Checks) - 1
				}
			} else {
				m.selected = 0
			}
		}

	case tickMsg:
		return m, tea.Batch(m.fetchCmd(), m.tickCmd())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func relativeTime(updatedAt string) string {
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func (m model) viewSelecting() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder
	maxWidth := m.width

	// Header
	b.WriteString(styleHeader.Render("  prtop"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Your recent open pull requests"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(styleFail.Render(truncate(fmt.Sprintf("Error: %s", m.err), maxWidth)))
		b.WriteString("\n\n")
		b.WriteString(styleDim.Render("r: retry | q: quit"))
		return b.String()
	}

	if m.loading {
		b.WriteString("Fetching your open PRs...")
		return b.String()
	}

	if len(m.prs) == 0 {
		b.WriteString("No open PRs found.")
		b.WriteString("\n\n")
		b.WriteString(styleDim.Render("r: retry | q: quit"))
		return b.String()
	}

	for idx, pr := range m.prs {
		isSelected := idx == m.selected
		marker := "  "
		if isSelected {
			marker = styleSelected.Render("▸ ")
		}

		// Line 1: marker + repo + #number
		repoStr := styleRepo.Render(pr.Repo)
		numStr := stylePRNumber.Render(fmt.Sprintf("#%d", pr.Number))
		line1 := marker + repoStr + " " + numStr

		// Line 2: title + updated timestamp
		titleStr := styleTitle.Render(pr.Title)
		updated := relativeTime(pr.UpdatedAt)
		line2 := "  " + titleStr
		if updated != "" {
			line2 += "  " + styleUpdatedAt.Render("updated "+updated)
		}

		if isSelected {
			b.WriteString(styleSelectedBg.Render(line1))
			b.WriteString("\n")
			b.WriteString(styleSelectedBg.Render(line2))
		} else {
			b.WriteString(line1)
			b.WriteString("\n")
			b.WriteString(line2)
		}
		b.WriteString("\n\n")
	}

	// Pad to bottom — each PR uses 3 lines (line1 + line2 + blank), header uses 3
	linesUsed := 3 + len(m.prs)*3
	for i := linesUsed; i < m.height-1; i++ {
		b.WriteString("\n")
	}

	b.WriteString(styleDim.Render(truncate("up/down: select | enter: view PR | q: quit", maxWidth)))

	return b.String()
}

func (m model) View() string {
	if m.mode == modeSelecting {
		return m.viewSelecting()
	}

	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder
	maxWidth := m.width

	// Header
	now := time.Now().Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("PR Checks - %s #%s", m.repo, m.prNumber)
	pad := maxWidth - len(header) - len(now)
	if pad < 1 {
		pad = 1
	}
	headerLine := header + strings.Repeat(" ", pad) + now
	b.WriteString(styleBold.Render(truncate(headerLine, maxWidth)))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(styleFail.Render(truncate(fmt.Sprintf("Error: %s", m.err), maxWidth)))
		b.WriteString("\n\n")
		b.WriteString(styleDim.Render("r: retry | q: quit"))
		return b.String()
	}

	if m.prData == nil {
		b.WriteString("\nFetching PR data...")
		return b.String()
	}

	// PR title
	if m.prData.Title != "" {
		b.WriteString(truncate(m.prData.Title, maxWidth))
		b.WriteString("\n")
	}

	// Branch + URL
	info := fmt.Sprintf("Branch: %s", m.prData.HeadRefName)
	if m.prData.URL != "" {
		info += fmt.Sprintf("    URL: %s", m.prData.URL)
	}
	b.WriteString(styleDim.Render(truncate(info, maxWidth)))
	b.WriteString("\n")

	// Blank line
	b.WriteString("\n")

	// Summary
	checks := m.prData.Checks
	total := len(checks)
	counts := map[CheckStatus]int{}
	for _, c := range checks {
		counts[c.Status]++
	}
	summary := fmt.Sprintf("Checks: %d total", total)
	var parts []string
	if n := counts[Pass]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", n))
	}
	if n := counts[Running]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d running", n))
	}
	if n := counts[Fail]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", n))
	}
	if n := counts[Skipped]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", n))
	}
	if len(parts) > 0 {
		summary += " - " + strings.Join(parts, ", ")
	}
	b.WriteString(styleBold.Render(truncate(summary, maxWidth)))
	b.WriteString("\n\n")

	// Table header
	statusW := 12
	durW := 12
	tableHdr := fmt.Sprintf("  %-*s%-*sNAME", statusW-2, "STATUS", durW, "DURATION")
	b.WriteString(styleUnder.Render(truncate(tableHdr, maxWidth)))
	b.WriteString("\n")

	// Calculate how many rows we can show
	// Lines used: header(1) + title(1) + branch(1) + blank(1) + summary(1) + blank(1) + table header(1) + footer(1) = 8
	maxRows := m.height - 8
	if maxRows < 1 {
		maxRows = 1
	}

	// Table rows
	for idx, check := range checks {
		if idx >= maxRows {
			break
		}

		// Compute live duration for running checks
		dur := check.Duration
		if !check.Completed && !check.StartedAt.IsZero() {
			delta := int(time.Since(check.StartedAt).Seconds())
			if delta < 0 {
				delta = 0
			}
			minutes := delta / 60
			seconds := delta % 60
			if minutes > 0 {
				dur = fmt.Sprintf("%dm%02ds", minutes, seconds)
			} else {
				dur = fmt.Sprintf("%ds", seconds)
			}
		}

		isSelected := idx == m.selected
		marker := "  "
		if isSelected {
			marker = "> "
		}

		statusStr := fmt.Sprintf("%s%-*s", marker, statusW-2, check.Status.String())
		durStr := fmt.Sprintf("%-*s", durW, dur)

		// Name column gets remaining width
		nameMaxW := maxWidth - statusW - durW
		if nameMaxW < 0 {
			nameMaxW = 0
		}
		nameStr := check.Name
		if len(nameStr) > nameMaxW {
			nameStr = nameStr[:nameMaxW]
		}

		// Apply status color
		var styledStatus string
		switch check.Status {
		case Pass:
			if isSelected {
				styledStatus = stylePass.Reverse(true).Render(statusStr)
			} else {
				styledStatus = stylePass.Render(statusStr)
			}
		case Fail:
			if isSelected {
				styledStatus = styleFail.Reverse(true).Render(statusStr)
			} else {
				styledStatus = styleFail.Render(statusStr)
			}
		case Running:
			if isSelected {
				styledStatus = styleRunning.Reverse(true).Render(statusStr)
			} else {
				styledStatus = styleRunning.Render(statusStr)
			}
		case Skipped:
			if isSelected {
				styledStatus = styleSkipped.Reverse(true).Render(statusStr)
			} else {
				styledStatus = styleSkipped.Render(statusStr)
			}
		}

		if isSelected {
			b.WriteString(styledStatus + styleReverse.Render(durStr+nameStr))
		} else {
			b.WriteString(styledStatus + durStr + nameStr)
		}
		b.WriteString("\n")
	}

	// Footer - pad to bottom of screen
	linesUsed := 7 + len(checks)
	if len(checks) > maxRows {
		linesUsed = 7 + maxRows
	}
	for i := linesUsed; i < m.height-1; i++ {
		b.WriteString("\n")
	}

	footer := fmt.Sprintf("Refresh: %ds | up/down: select | enter: open | r: refresh | q: quit",
		int(m.interval.Seconds()))
	b.WriteString(styleDim.Render(truncate(footer, maxWidth)))

	return b.String()
}

func truncate(s string, maxWidth int) string {
	if len(s) > maxWidth && maxWidth > 0 {
		return s[:maxWidth]
	}
	return s
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

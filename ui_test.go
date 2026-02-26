package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// relativeTime
// ---------------------------------------------------------------------------

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name      string
		updatedAt string
		want      string
	}{
		{
			name:      "just now",
			updatedAt: time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339),
			want:      "just now",
		},
		{
			name:      "minutes ago",
			updatedAt: time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339),
			want:      "5m ago",
		},
		{
			name:      "hours ago",
			updatedAt: time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
			want:      "2h ago",
		},
		{
			name:      "days ago",
			updatedAt: time.Now().UTC().Add(-3 * 24 * time.Hour).Format(time.RFC3339),
			want:      "3d ago",
		},
		{
			name:      "invalid timestamp",
			updatedAt: "not-a-timestamp",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(tt.updatedAt)
			if got != tt.want {
				t.Errorf("relativeTime(%q) = %q, want %q", tt.updatedAt, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"longer than max", "hello world", 5, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"maxWidth 0", "hello", 0, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// newModel / newSelectModel
// ---------------------------------------------------------------------------

func TestNewModel(t *testing.T) {
	m := newModel("owner/repo", "42", 5*time.Second)
	if m.mode != modeViewing {
		t.Errorf("mode = %v, want modeViewing", m.mode)
	}
	if m.repo != "owner/repo" {
		t.Errorf("repo = %q, want %q", m.repo, "owner/repo")
	}
	if m.prNumber != "42" {
		t.Errorf("prNumber = %q, want %q", m.prNumber, "42")
	}
	if m.interval != 5*time.Second {
		t.Errorf("interval = %v, want %v", m.interval, 5*time.Second)
	}
}

func TestNewSelectModel(t *testing.T) {
	m := newSelectModel(10 * time.Second)
	if m.mode != modeSelecting {
		t.Errorf("mode = %v, want modeSelecting", m.mode)
	}
	if !m.loading {
		t.Error("loading should be true")
	}
	if m.interval != 10*time.Second {
		t.Errorf("interval = %v, want %v", m.interval, 10*time.Second)
	}
}

// ---------------------------------------------------------------------------
// model.Update
// ---------------------------------------------------------------------------

func TestModelUpdate(t *testing.T) {
	t.Run("KeyDown in selecting mode", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.prs = []PRSummary{{Repo: "a"}, {Repo: "b"}, {Repo: "c"}}
		m.selected = 0

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		um := updated.(model)
		if um.selected != 1 {
			t.Errorf("selected = %d, want 1", um.selected)
		}
	})

	t.Run("KeyDown clamps at end in selecting mode", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.prs = []PRSummary{{Repo: "a"}, {Repo: "b"}}
		m.selected = 1

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		um := updated.(model)
		if um.selected != 1 {
			t.Errorf("selected = %d, want 1 (clamped)", um.selected)
		}
	})

	t.Run("KeyUp clamps at zero", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.selected = 0

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		um := updated.(model)
		if um.selected != 0 {
			t.Errorf("selected = %d, want 0 (clamped)", um.selected)
		}
	})

	t.Run("j/k navigation in viewing mode", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.prData = &PRData{Checks: []Check{{Name: "a"}, {Name: "b"}, {Name: "c"}}}
		m.selected = 0

		// j moves down
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		um := updated.(model)
		if um.selected != 1 {
			t.Errorf("after j: selected = %d, want 1", um.selected)
		}

		// k moves up
		updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		um = updated.(model)
		if um.selected != 0 {
			t.Errorf("after k: selected = %d, want 0", um.selected)
		}
	})

	t.Run("j clamps at end in viewing mode", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.prData = &PRData{Checks: []Check{{Name: "a"}, {Name: "b"}}}
		m.selected = 1

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		um := updated.(model)
		if um.selected != 1 {
			t.Errorf("selected = %d, want 1 (clamped)", um.selected)
		}
	})

	t.Run("Ctrl+C returns quit", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Fatal("expected quit cmd, got nil")
		}
		// Execute the cmd to verify it produces a quit message
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected tea.QuitMsg, got %T", msg)
		}
	})

	t.Run("q returns quit", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if cmd == nil {
			t.Fatal("expected quit cmd, got nil")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected tea.QuitMsg, got %T", msg)
		}
	})

	t.Run("r in selecting mode sets loading", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.loading = false
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		um := updated.(model)
		if !um.loading {
			t.Error("loading should be true after 'r'")
		}
		if cmd == nil {
			t.Error("expected cmd for fetchPRList")
		}
	})

	t.Run("r in viewing mode returns fetchCmd", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		if cmd == nil {
			t.Error("expected cmd for fetch")
		}
	})

	t.Run("Enter in selecting mode transitions to viewing", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.prs = []PRSummary{
			{Repo: "owner/repo", Number: 42, Title: "Test PR"},
			{Repo: "other/proj", Number: 99, Title: "Other"},
		}
		m.selected = 1

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		um := updated.(model)
		if um.mode != modeViewing {
			t.Errorf("mode = %v, want modeViewing", um.mode)
		}
		if um.repo != "other/proj" {
			t.Errorf("repo = %q, want %q", um.repo, "other/proj")
		}
		if um.prNumber != "99" {
			t.Errorf("prNumber = %q, want %q", um.prNumber, "99")
		}
		if um.selected != 0 {
			t.Errorf("selected = %d, want 0 (reset)", um.selected)
		}
		if um.prData != nil {
			t.Error("prData should be nil after transition")
		}
		if cmd == nil {
			t.Error("expected batch cmd after transition")
		}
	})

	t.Run("Enter in selecting mode with empty list", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.prs = []PRSummary{}

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		um := updated.(model)
		if um.mode != modeSelecting {
			t.Errorf("mode = %v, want modeSelecting (no transition)", um.mode)
		}
		if cmd != nil {
			t.Error("expected nil cmd when no PRs to select")
		}
	})

	t.Run("prListMsg sets prs", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.loading = true
		m.selected = 2

		prs := []PRSummary{{Repo: "a"}, {Repo: "b"}}
		updated, _ := m.Update(prListMsg{prs: prs})
		um := updated.(model)
		if um.loading {
			t.Error("loading should be false")
		}
		if len(um.prs) != 2 {
			t.Errorf("got %d prs, want 2", len(um.prs))
		}
		if um.selected != 0 {
			t.Errorf("selected = %d, want 0 (reset)", um.selected)
		}
		if um.err != nil {
			t.Errorf("err should be nil, got %v", um.err)
		}
	})

	t.Run("prListMsg with error", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.loading = true

		updated, _ := m.Update(prListMsg{err: fmt.Errorf("network error")})
		um := updated.(model)
		if um.loading {
			t.Error("loading should be false")
		}
		if um.err == nil {
			t.Error("err should be set")
		}
	})

	t.Run("prDataMsg sets prData and clamps selected", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.selected = 5

		data := &PRData{Checks: []Check{{Name: "a"}, {Name: "b"}}}
		updated, _ := m.Update(prDataMsg{data: data})
		um := updated.(model)
		if um.prData == nil {
			t.Fatal("prData should be set")
		}
		if um.selected != 1 {
			t.Errorf("selected = %d, want 1 (clamped to len-1)", um.selected)
		}
		if um.err != nil {
			t.Errorf("err should be nil, got %v", um.err)
		}
	})

	t.Run("prDataMsg with empty checks resets selected", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.selected = 3

		data := &PRData{Checks: []Check{}}
		updated, _ := m.Update(prDataMsg{data: data})
		um := updated.(model)
		if um.selected != 0 {
			t.Errorf("selected = %d, want 0", um.selected)
		}
	})

	t.Run("prDataMsg with error", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)

		updated, _ := m.Update(prDataMsg{err: fmt.Errorf("fetch failed")})
		um := updated.(model)
		if um.err == nil {
			t.Error("err should be set")
		}
	})

	t.Run("WindowSizeMsg sets dimensions", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		um := updated.(model)
		if um.width != 120 {
			t.Errorf("width = %d, want 120", um.width)
		}
		if um.height != 40 {
			t.Errorf("height = %d, want 40", um.height)
		}
	})
}

// ---------------------------------------------------------------------------
// viewSelecting
// ---------------------------------------------------------------------------

func TestViewSelecting(t *testing.T) {
	t.Run("width=0 shows Loading", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.width = 0
		out := m.viewSelecting()
		if out != "Loading..." {
			t.Errorf("got %q, want %q", out, "Loading...")
		}
	})

	t.Run("loading=true shows fetching message", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.width = 80
		m.loading = true
		out := m.viewSelecting()
		if !strings.Contains(out, "Fetching your open PRs...") {
			t.Errorf("output should contain fetching message, got %q", out)
		}
	})

	t.Run("error shows Error", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.width = 80
		m.loading = false
		m.err = fmt.Errorf("something went wrong")
		out := m.viewSelecting()
		if !strings.Contains(out, "Error:") {
			t.Errorf("output should contain 'Error:', got %q", out)
		}
	})

	t.Run("empty prs shows no PRs found", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.width = 80
		m.loading = false
		m.prs = []PRSummary{}
		out := m.viewSelecting()
		if !strings.Contains(out, "No open PRs found.") {
			t.Errorf("output should contain 'No open PRs found.', got %q", out)
		}
	})

	t.Run("with PRs shows repo and number", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.width = 80
		m.height = 30
		m.loading = false
		m.prs = []PRSummary{
			{Repo: "owner/repo", Number: 42, Title: "My PR"},
			{Repo: "other/proj", Number: 99, Title: "Other PR"},
		}
		m.selected = 0
		out := m.viewSelecting()
		if !strings.Contains(out, "owner/repo") {
			t.Error("output should contain repo name")
		}
		if !strings.Contains(out, "#42") {
			t.Error("output should contain PR number")
		}
		if !strings.Contains(out, "My PR") {
			t.Error("output should contain PR title")
		}
		if !strings.Contains(out, "other/proj") {
			t.Error("output should contain second repo name")
		}
	})

	t.Run("selected item has marker", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.width = 80
		m.height = 30
		m.loading = false
		m.prs = []PRSummary{
			{Repo: "owner/repo", Number: 42, Title: "My PR"},
		}
		m.selected = 0
		out := m.viewSelecting()
		if !strings.Contains(out, "▸") {
			t.Error("output should contain selection marker '▸'")
		}
	})
}

// ---------------------------------------------------------------------------
// View (viewing mode)
// ---------------------------------------------------------------------------

func TestView(t *testing.T) {
	t.Run("width=0 shows Loading", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.width = 0
		out := m.View()
		if out != "Loading..." {
			t.Errorf("got %q, want %q", out, "Loading...")
		}
	})

	t.Run("nil prData shows fetching", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.width = 80
		m.height = 30
		out := m.View()
		if !strings.Contains(out, "Fetching PR data...") {
			t.Errorf("output should contain 'Fetching PR data...', got %q", out)
		}
	})

	t.Run("error shows Error", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.width = 80
		m.height = 30
		m.err = fmt.Errorf("bad request")
		out := m.View()
		if !strings.Contains(out, "Error:") {
			t.Errorf("output should contain 'Error:', got %q", out)
		}
	})

	t.Run("with checks shows check names and summary", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.width = 120
		m.height = 30
		m.prData = &PRData{
			Title:       "Test PR Title",
			HeadRefName: "feature",
			URL:         "https://github.com/o/r/pull/1",
			Checks: []Check{
				{Name: "build", Status: Pass, Duration: "1m30s", Completed: true},
				{Name: "lint", Status: Fail, Duration: "20s", Completed: true},
				{Name: "deploy", Status: Running, Duration: "5s", Completed: false},
			},
		}
		out := m.View()

		// Check names present
		if !strings.Contains(out, "build") {
			t.Error("output should contain check name 'build'")
		}
		if !strings.Contains(out, "lint") {
			t.Error("output should contain check name 'lint'")
		}
		if !strings.Contains(out, "deploy") {
			t.Error("output should contain check name 'deploy'")
		}

		// Status strings
		if !strings.Contains(out, "PASS") {
			t.Error("output should contain 'PASS'")
		}
		if !strings.Contains(out, "FAIL") {
			t.Error("output should contain 'FAIL'")
		}
		if !strings.Contains(out, "RUNNING") {
			t.Error("output should contain 'RUNNING'")
		}

		// Summary counts
		if !strings.Contains(out, "3 total") {
			t.Error("output should contain '3 total'")
		}
		if !strings.Contains(out, "1 passed") {
			t.Error("output should contain '1 passed'")
		}
		if !strings.Contains(out, "1 failed") {
			t.Error("output should contain '1 failed'")
		}
		if !strings.Contains(out, "1 running") {
			t.Error("output should contain '1 running'")
		}

		// PR metadata
		if !strings.Contains(out, "Test PR Title") {
			t.Error("output should contain PR title")
		}
		if !strings.Contains(out, "feature") {
			t.Error("output should contain branch name")
		}
	})
}

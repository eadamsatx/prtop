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
	if !m.hideSkipped {
		t.Error("hideSkipped should default to true")
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
	if !m.canGoBack {
		t.Error("canGoBack should be true")
	}
	if m.interval != 10*time.Second {
		t.Errorf("interval = %v, want %v", m.interval, 10*time.Second)
	}
	if !m.hideSkipped {
		t.Error("hideSkipped should default to true")
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

	t.Run("s toggles hideSkipped", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.hideSkipped = true
		m.selected = 3
		m.scrollOff = 2

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		um := updated.(model)
		if um.hideSkipped {
			t.Error("hideSkipped should be false after toggle")
		}
		if um.selected != 0 {
			t.Errorf("selected = %d, want 0 (reset)", um.selected)
		}
		if um.scrollOff != 0 {
			t.Errorf("scrollOff = %d, want 0 (reset)", um.scrollOff)
		}

		// Toggle back
		updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		um = updated.(model)
		if !um.hideSkipped {
			t.Error("hideSkipped should be true after second toggle")
		}
	})

	t.Run("s does nothing in selecting mode", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.hideSkipped = true

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		um := updated.(model)
		if !um.hideSkipped {
			t.Error("hideSkipped should remain true in selecting mode")
		}
	})

	t.Run("navigation with filtering clamps to filtered length", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.height = 40
		m.prData = &PRData{Checks: []Check{
			{Name: "build", Status: Pass},
			{Name: "skip1", Status: Skipped},
			{Name: "lint", Status: Fail},
			{Name: "skip2", Status: Skipped},
		}}
		m.hideSkipped = true
		m.selected = 1 // at last filtered item (build, lint → 2 items, idx 0,1)

		// j should not go beyond 1 (len(filtered)-1)
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		um := updated.(model)
		if um.selected != 1 {
			t.Errorf("selected = %d, want 1 (clamped to filtered len-1)", um.selected)
		}
	})

	t.Run("Enter with filtering opens correct check", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.height = 40
		m.prData = &PRData{Checks: []Check{
			{Name: "build", Status: Pass, DetailsURL: "http://build"},
			{Name: "skip1", Status: Skipped, DetailsURL: "http://skip1"},
			{Name: "lint", Status: Fail, DetailsURL: "http://lint"},
		}}
		m.hideSkipped = true
		m.selected = 1 // should be "lint" in filtered view

		// We can't easily test the browser opening, but we can verify
		// the filtered list indexes correctly
		checks := m.filteredChecks()
		if len(checks) != 2 {
			t.Fatalf("filtered checks = %d, want 2", len(checks))
		}
		if checks[1].Name != "lint" {
			t.Errorf("filtered[1].Name = %q, want %q", checks[1].Name, "lint")
		}
	})

	t.Run("prDataMsg clamps to filtered length", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.height = 40
		m.hideSkipped = true
		m.selected = 5

		data := &PRData{Checks: []Check{
			{Name: "build", Status: Pass},
			{Name: "skip1", Status: Skipped},
			{Name: "skip2", Status: Skipped},
		}}
		updated, _ := m.Update(prDataMsg{data: data})
		um := updated.(model)
		// Only 1 non-skipped check, so selected should clamp to 0
		if um.selected != 0 {
			t.Errorf("selected = %d, want 0 (clamped to filtered len-1)", um.selected)
		}
	})

	t.Run("Esc in viewing mode with canGoBack returns to selecting", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		// Simulate having selected a PR and transitioned to viewing
		m.mode = modeViewing
		m.repo = "owner/repo"
		m.prNumber = "42"
		m.prData = &PRData{Checks: []Check{{Name: "a"}}}
		m.selected = 1
		m.scrollOff = 1
		m.loading = false

		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		um := updated.(model)
		if um.mode != modeSelecting {
			t.Errorf("mode = %v, want modeSelecting", um.mode)
		}
		if um.selected != 0 {
			t.Errorf("selected = %d, want 0 (reset)", um.selected)
		}
		if um.scrollOff != 0 {
			t.Errorf("scrollOff = %d, want 0 (reset)", um.scrollOff)
		}
		if um.prData != nil {
			t.Error("prData should be nil after going back")
		}
		if !um.loading {
			t.Error("loading should be true after going back")
		}
		if cmd == nil {
			t.Error("expected cmd for fetchPRList")
		}
	})

	t.Run("Esc in viewing mode without canGoBack does nothing", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.prData = &PRData{Checks: []Check{{Name: "a"}}}

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		um := updated.(model)
		if um.mode != modeViewing {
			t.Errorf("mode = %v, want modeViewing (no back)", um.mode)
		}
	})

	t.Run("Esc in selecting mode does nothing", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.loading = false
		m.prs = []PRSummary{{Repo: "a"}}

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		um := updated.(model)
		if um.mode != modeSelecting {
			t.Errorf("mode = %v, want modeSelecting", um.mode)
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

	t.Run("with filtering active skipped checks not shown", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.width = 120
		m.height = 30
		m.hideSkipped = true
		m.prData = &PRData{
			Title:       "Test PR",
			HeadRefName: "feature",
			Checks: []Check{
				{Name: "build", Status: Pass, Duration: "1m30s", Completed: true},
				{Name: "skip-me", Status: Skipped, Duration: "", Completed: true},
				{Name: "lint", Status: Fail, Duration: "20s", Completed: true},
			},
		}
		out := m.View()

		// Non-skipped checks should be present
		if !strings.Contains(out, "build") {
			t.Error("output should contain 'build'")
		}
		if !strings.Contains(out, "lint") {
			t.Error("output should contain 'lint'")
		}
		// Skipped check should NOT be present
		if strings.Contains(out, "skip-me") {
			t.Error("output should not contain 'skip-me' when filtering is active")
		}
		// Summary should show hidden count
		if !strings.Contains(out, "(1 hidden)") {
			t.Error("output should contain '(1 hidden)'")
		}
		// Summary should still show total from unfiltered
		if !strings.Contains(out, "3 total") {
			t.Error("output should contain '3 total' from unfiltered checks")
		}
	})

	t.Run("footer shows correct filter toggle text", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.width = 120
		m.height = 30
		m.hideSkipped = true
		m.prData = &PRData{
			Title:       "Test PR",
			HeadRefName: "feature",
			Checks:      []Check{{Name: "a", Status: Pass}},
		}
		out := m.View()
		if !strings.Contains(out, "s: show skipped") {
			t.Error("footer should contain 's: show skipped' when hideSkipped=true")
		}

		m.hideSkipped = false
		out = m.View()
		if !strings.Contains(out, "s: hide skipped") {
			t.Error("footer should contain 's: hide skipped' when hideSkipped=false")
		}
	})

	t.Run("footer shows esc hint when canGoBack", func(t *testing.T) {
		m := newSelectModel(5 * time.Second)
		m.mode = modeViewing
		m.width = 120
		m.height = 30
		m.prData = &PRData{
			Title:       "Test PR",
			HeadRefName: "feature",
			Checks:      []Check{{Name: "a", Status: Pass}},
		}
		out := m.View()
		if !strings.Contains(out, "esc: back") {
			t.Error("footer should contain 'esc: back' when canGoBack=true")
		}

		// Without canGoBack, no esc hint
		m.canGoBack = false
		out = m.View()
		if strings.Contains(out, "esc: back") {
			t.Error("footer should not contain 'esc: back' when canGoBack=false")
		}
	})
}

// ---------------------------------------------------------------------------
// filteredChecks
// ---------------------------------------------------------------------------

func TestFilteredChecks(t *testing.T) {
	t.Run("nil prData returns nil", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.prData = nil
		got := m.filteredChecks()
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("hideSkipped=false returns all checks", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.hideSkipped = false
		m.prData = &PRData{Checks: []Check{
			{Name: "a", Status: Pass},
			{Name: "b", Status: Skipped},
			{Name: "c", Status: Fail},
		}}
		got := m.filteredChecks()
		if len(got) != 3 {
			t.Errorf("len = %d, want 3", len(got))
		}
	})

	t.Run("hideSkipped=true excludes Skipped checks", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.hideSkipped = true
		m.prData = &PRData{Checks: []Check{
			{Name: "a", Status: Pass},
			{Name: "b", Status: Skipped},
			{Name: "c", Status: Fail},
			{Name: "d", Status: Skipped},
		}}
		got := m.filteredChecks()
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
		if got[0].Name != "a" {
			t.Errorf("got[0].Name = %q, want %q", got[0].Name, "a")
		}
		if got[1].Name != "c" {
			t.Errorf("got[1].Name = %q, want %q", got[1].Name, "c")
		}
	})

	t.Run("all skipped returns empty slice", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.hideSkipped = true
		m.prData = &PRData{Checks: []Check{
			{Name: "a", Status: Skipped},
			{Name: "b", Status: Skipped},
		}}
		got := m.filteredChecks()
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// scroll offset
// ---------------------------------------------------------------------------

func TestScrollOffset(t *testing.T) {
	t.Run("selected beyond viewport adjusts scrollOff", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.height = 12 // maxRows = 12 - 8 = 4
		m.hideSkipped = false
		m.prData = &PRData{Checks: []Check{
			{Name: "a", Status: Pass},
			{Name: "b", Status: Pass},
			{Name: "c", Status: Pass},
			{Name: "d", Status: Pass},
			{Name: "e", Status: Pass},
			{Name: "f", Status: Pass},
		}}
		m.selected = 0
		m.scrollOff = 0

		// Navigate down past viewport
		for i := 0; i < 5; i++ {
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = updated.(model)
		}
		if m.selected != 5 {
			t.Errorf("selected = %d, want 5", m.selected)
		}
		// scrollOff should have adjusted: selected(5) >= scrollOff + maxRows(4)
		// so scrollOff = 5 - 4 + 1 = 2
		if m.scrollOff != 2 {
			t.Errorf("scrollOff = %d, want 2", m.scrollOff)
		}
	})

	t.Run("selected above viewport adjusts scrollOff", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.height = 12 // maxRows = 4
		m.hideSkipped = false
		m.prData = &PRData{Checks: []Check{
			{Name: "a", Status: Pass},
			{Name: "b", Status: Pass},
			{Name: "c", Status: Pass},
			{Name: "d", Status: Pass},
			{Name: "e", Status: Pass},
		}}
		m.selected = 2
		m.scrollOff = 2

		// Navigate up past scroll offset
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(model)
		if m.selected != 1 {
			t.Errorf("selected = %d, want 1", m.selected)
		}
		if m.scrollOff != 1 {
			t.Errorf("scrollOff = %d, want 1", m.scrollOff)
		}
	})

	t.Run("scrollOff stays 0 when list fits in viewport", func(t *testing.T) {
		m := newModel("o/r", "1", 5*time.Second)
		m.height = 30 // maxRows = 22, much more than 3 checks
		m.hideSkipped = false
		m.prData = &PRData{Checks: []Check{
			{Name: "a", Status: Pass},
			{Name: "b", Status: Pass},
			{Name: "c", Status: Pass},
		}}
		m.selected = 0
		m.scrollOff = 0

		// Navigate through all items
		for i := 0; i < 2; i++ {
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = updated.(model)
		}
		if m.selected != 2 {
			t.Errorf("selected = %d, want 2", m.selected)
		}
		if m.scrollOff != 0 {
			t.Errorf("scrollOff = %d, want 0 (list fits in viewport)", m.scrollOff)
		}
	})
}

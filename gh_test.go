package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// CheckStatus.String()
// ---------------------------------------------------------------------------

func TestCheckStatusString(t *testing.T) {
	tests := []struct {
		status CheckStatus
		want   string
	}{
		{Running, "RUNNING"},
		{Fail, "FAIL"},
		{Pass, "PASS"},
		{Skipped, "SKIPPED"},
		{CheckStatus(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("CheckStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// normalizeStatus
// ---------------------------------------------------------------------------

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input string
		want  CheckStatus
	}{
		// Pass
		{"SUCCESS", Pass},
		{"PASS", Pass},
		// Fail
		{"FAILURE", Fail},
		{"FAIL", Fail},
		{"ERROR", Fail},
		{"TIMED_OUT", Fail},
		{"ACTION_REQUIRED", Fail},
		{"STARTUP_FAILURE", Fail},
		// Running
		{"IN_PROGRESS", Running},
		{"RUNNING", Running},
		{"PENDING", Running},
		{"QUEUED", Running},
		{"WAITING", Running},
		{"REQUESTED", Running},
		// Skipped
		{"SKIPPED", Skipped},
		{"CANCELLED", Skipped},
		{"NEUTRAL", Skipped},
		{"STALE", Skipped},
		// Case insensitivity
		{"success", Pass},
		{"Success", Pass},
		{"failure", Fail},
		// Whitespace trimming
		{"  SUCCESS  ", Pass},
		{"\tFAILURE\n", Fail},
		// Empty → Running
		{"", Running},
		// Unknown → Running
		{"SOMETHING_ELSE", Running},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			if got := normalizeStatus(tt.input); got != tt.want {
				t.Errorf("normalizeStatus(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseDuration
// ---------------------------------------------------------------------------

func TestParseDuration(t *testing.T) {
	t.Run("completed check", func(t *testing.T) {
		start := "2024-01-01T10:00:00Z"
		end := "2024-01-01T10:02:35Z"
		dur, startTime, completed := parseDuration(start, end)
		if dur != "2m35s" {
			t.Errorf("duration = %q, want %q", dur, "2m35s")
		}
		if startTime.IsZero() {
			t.Error("startTime should not be zero")
		}
		if !completed {
			t.Error("completed should be true")
		}
	})

	t.Run("short duration under 60s", func(t *testing.T) {
		start := "2024-01-01T10:00:00Z"
		end := "2024-01-01T10:00:45Z"
		dur, _, completed := parseDuration(start, end)
		if dur != "45s" {
			t.Errorf("duration = %q, want %q", dur, "45s")
		}
		if !completed {
			t.Error("completed should be true")
		}
	})

	t.Run("empty startedAt", func(t *testing.T) {
		dur, startTime, completed := parseDuration("", "2024-01-01T10:00:00Z")
		if dur != "-" {
			t.Errorf("duration = %q, want %q", dur, "-")
		}
		if !startTime.IsZero() {
			t.Error("startTime should be zero")
		}
		if completed {
			t.Error("completed should be false")
		}
	})

	t.Run("invalid startedAt", func(t *testing.T) {
		dur, startTime, completed := parseDuration("not-a-date", "2024-01-01T10:00:00Z")
		if dur != "-" {
			t.Errorf("duration = %q, want %q", dur, "-")
		}
		if !startTime.IsZero() {
			t.Error("startTime should be zero")
		}
		if completed {
			t.Error("completed should be false")
		}
	})

	t.Run("empty completedAt uses time.Now", func(t *testing.T) {
		// Use a start time very close to now
		start := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
		dur, startTime, completed := parseDuration(start, "")
		if startTime.IsZero() {
			t.Error("startTime should not be zero")
		}
		if completed {
			t.Error("completed should be false when completedAt is empty")
		}
		// Duration should be roughly 10s (give or take a second)
		if dur == "-" {
			t.Error("duration should not be '-'")
		}
	})
}

// ---------------------------------------------------------------------------
// formatDuration
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{0, "0s"},
		{5, "5s"},
		{59, "59s"},
		{60, "1m00s"},
		{90, "1m30s"},
		{155, "2m35s"},
		{3600, "60m00s"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatDuration(tt.seconds); got != tt.want {
				t.Errorf("formatDuration(%d) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// exec mock helpers
// ---------------------------------------------------------------------------

// fakeExecCommand returns an exec.Cmd that, when run, invokes this test binary
// via TestHelperProcess with the given stdout/stderr/exit code.
func fakeExecCommand(stdout string, stderr string, exitCode int) func(string, ...string) *exec.Cmd {
	return func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_TEST_HELPER_PROCESS=1",
			fmt.Sprintf("GO_TEST_HELPER_STDOUT=%s", stdout),
			fmt.Sprintf("GO_TEST_HELPER_STDERR=%s", stderr),
			fmt.Sprintf("GO_TEST_HELPER_EXIT=%d", exitCode),
		)
		return cmd
	}
}

// TestHelperProcess is the subprocess entry point used by fakeExecCommand.
// It is not a real test.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	stdout := os.Getenv("GO_TEST_HELPER_STDOUT")
	stderr := os.Getenv("GO_TEST_HELPER_STDERR")
	exitCode := 0
	fmt.Sscanf(os.Getenv("GO_TEST_HELPER_EXIT"), "%d", &exitCode)

	fmt.Fprint(os.Stdout, stdout)
	fmt.Fprint(os.Stderr, stderr)
	os.Exit(exitCode)
}

// ---------------------------------------------------------------------------
// fetchRecentPRs
// ---------------------------------------------------------------------------

func TestFetchRecentPRs(t *testing.T) {
	t.Run("success with 2 PRs", func(t *testing.T) {
		json := `[
			{"number":42,"title":"Add feature","repository":{"nameWithOwner":"owner/repo"},"url":"https://github.com/owner/repo/pull/42","updatedAt":"2024-01-01T00:00:00Z"},
			{"number":99,"title":"Fix bug","repository":{"nameWithOwner":"other/project"},"url":"https://github.com/other/project/pull/99","updatedAt":"2024-01-02T00:00:00Z"}
		]`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		prs, err := fetchRecentPRs()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prs) != 2 {
			t.Fatalf("got %d PRs, want 2", len(prs))
		}
		if prs[0].Repo != "owner/repo" {
			t.Errorf("prs[0].Repo = %q, want %q", prs[0].Repo, "owner/repo")
		}
		if prs[0].Number != 42 {
			t.Errorf("prs[0].Number = %d, want 42", prs[0].Number)
		}
		if prs[0].Title != "Add feature" {
			t.Errorf("prs[0].Title = %q, want %q", prs[0].Title, "Add feature")
		}
		if prs[1].Repo != "other/project" {
			t.Errorf("prs[1].Repo = %q, want %q", prs[1].Repo, "other/project")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		execCommand = fakeExecCommand("[]", "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		prs, err := fetchRecentPRs()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prs) != 0 {
			t.Errorf("got %d PRs, want 0", len(prs))
		}
	})

	t.Run("gh CLI error", func(t *testing.T) {
		execCommand = fakeExecCommand("", "gh: not logged in", 1)
		t.Cleanup(func() { execCommand = exec.Command })

		_, err := fetchRecentPRs()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "gh: not logged in") {
			t.Errorf("error = %q, should contain stderr message", err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		execCommand = fakeExecCommand("{invalid json", "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		_, err := fetchRecentPRs()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse") {
			t.Errorf("error = %q, should contain 'failed to parse'", err)
		}
	})
}

// ---------------------------------------------------------------------------
// fetchPRData
// ---------------------------------------------------------------------------

func TestFetchPRData(t *testing.T) {
	t.Run("success with CheckRun items", func(t *testing.T) {
		json := `{
			"title": "My PR",
			"headRefName": "feature-branch",
			"url": "https://github.com/owner/repo/pull/1",
			"statusCheckRollup": [
				{
					"__typename": "CheckRun",
					"name": "build",
					"status": "COMPLETED",
					"conclusion": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:01:30Z",
					"detailsUrl": "https://example.com/build",
					"workflowName": "CI"
				},
				{
					"__typename": "CheckRun",
					"name": "lint",
					"status": "COMPLETED",
					"conclusion": "FAILURE",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:00:20Z",
					"detailsUrl": "https://example.com/lint",
					"workflowName": ""
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("owner/repo", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.Title != "My PR" {
			t.Errorf("Title = %q, want %q", data.Title, "My PR")
		}
		if data.HeadRefName != "feature-branch" {
			t.Errorf("HeadRefName = %q, want %q", data.HeadRefName, "feature-branch")
		}
		if len(data.Checks) != 2 {
			t.Fatalf("got %d checks, want 2", len(data.Checks))
		}
		// Sorted by status: Fail < Pass, so lint (Fail) comes first
		if data.Checks[0].Name != "lint" {
			t.Errorf("checks[0].Name = %q, want %q (sorted by status)", data.Checks[0].Name, "lint")
		}
		if data.Checks[0].Status != Fail {
			t.Errorf("checks[0].Status = %v, want Fail", data.Checks[0].Status)
		}
		if data.Checks[1].Name != "build (CI)" {
			t.Errorf("checks[1].Name = %q, want %q", data.Checks[1].Name, "build (CI)")
		}
		if data.Checks[1].Status != Pass {
			t.Errorf("checks[1].Status = %v, want Pass", data.Checks[1].Status)
		}
		if data.Checks[1].Duration != "1m30s" {
			t.Errorf("checks[1].Duration = %q, want %q", data.Checks[1].Duration, "1m30s")
		}
		if data.Checks[1].DetailsURL != "https://example.com/build" {
			t.Errorf("checks[1].DetailsURL = %q, want %q", data.Checks[1].DetailsURL, "https://example.com/build")
		}
	})

	t.Run("StatusContext item", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "https://github.com/o/r/pull/1",
			"statusCheckRollup": [
				{
					"__typename": "StatusContext",
					"context": "ci/jenkins",
					"state": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": ""
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data.Checks) != 1 {
			t.Fatalf("got %d checks, want 1", len(data.Checks))
		}
		c := data.Checks[0]
		if c.Name != "ci/jenkins" {
			t.Errorf("Name = %q, want %q", c.Name, "ci/jenkins")
		}
		if c.Status != Pass {
			t.Errorf("Status = %v, want Pass", c.Status)
		}
		if c.Duration != "???" {
			t.Errorf("Duration = %q, want %q", c.Duration, "???")
		}
		if !c.Completed {
			t.Error("Completed should be true for StatusContext with non-Running status")
		}
	})

	t.Run("workflow name appended", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "",
			"statusCheckRollup": [
				{
					"__typename": "CheckRun",
					"name": "test",
					"workflowName": "Tests",
					"conclusion": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:00:10Z"
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.Checks[0].Name != "test (Tests)" {
			t.Errorf("Name = %q, want %q", data.Checks[0].Name, "test (Tests)")
		}
	})

	t.Run("name fallback to context then unknown", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "",
			"statusCheckRollup": [
				{
					"__typename": "StatusContext",
					"name": "",
					"context": "ctx-name",
					"state": "PENDING",
					"startedAt": "2024-01-01T10:00:00Z"
				},
				{
					"__typename": "CheckRun",
					"name": "",
					"context": "",
					"conclusion": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:00:05Z"
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Sorted: Running (ctx-name) before Pass (unknown)
		names := []string{data.Checks[0].Name, data.Checks[1].Name}
		if names[0] != "ctx-name" {
			t.Errorf("checks[0].Name = %q, want %q", names[0], "ctx-name")
		}
		if names[1] != "unknown" {
			t.Errorf("checks[1].Name = %q, want %q", names[1], "unknown")
		}
	})

	t.Run("conclusion takes priority over status", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "",
			"statusCheckRollup": [
				{
					"__typename": "CheckRun",
					"name": "check1",
					"status": "COMPLETED",
					"conclusion": "FAILURE",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:00:05Z"
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.Checks[0].Status != Fail {
			t.Errorf("Status = %v, want Fail (conclusion should take priority)", data.Checks[0].Status)
		}
	})

	t.Run("status over state when no conclusion", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "",
			"statusCheckRollup": [
				{
					"__typename": "CheckRun",
					"name": "check1",
					"status": "IN_PROGRESS",
					"conclusion": "",
					"state": "PENDING",
					"startedAt": "2024-01-01T10:00:00Z"
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.Checks[0].Status != Running {
			t.Errorf("Status = %v, want Running (status should take priority over state)", data.Checks[0].Status)
		}
	})

	t.Run("zero-value completedAt treated as empty", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "",
			"statusCheckRollup": [
				{
					"__typename": "CheckRun",
					"name": "check1",
					"conclusion": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "0001-01-01T00:00:00Z"
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// completedAt should be treated as empty, so completed = false
		if data.Checks[0].Completed {
			t.Error("Completed should be false when completedAt is zero-value")
		}
	})

	t.Run("detailsUrl fallback to targetUrl", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "",
			"statusCheckRollup": [
				{
					"__typename": "StatusContext",
					"context": "ci",
					"state": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"detailsUrl": "",
					"targetUrl": "https://jenkins.example.com/job/123"
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.Checks[0].DetailsURL != "https://jenkins.example.com/job/123" {
			t.Errorf("DetailsURL = %q, want targetUrl fallback", data.Checks[0].DetailsURL)
		}
	})

	t.Run("checks sorted by status then name", func(t *testing.T) {
		json := `{
			"title": "PR",
			"headRefName": "main",
			"url": "",
			"statusCheckRollup": [
				{
					"__typename": "CheckRun",
					"name": "zebra",
					"conclusion": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:00:05Z"
				},
				{
					"__typename": "CheckRun",
					"name": "alpha",
					"conclusion": "SUCCESS",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:00:05Z"
				},
				{
					"__typename": "CheckRun",
					"name": "beta",
					"conclusion": "FAILURE",
					"startedAt": "2024-01-01T10:00:00Z",
					"completedAt": "2024-01-01T10:00:05Z"
				}
			]
		}`
		execCommand = fakeExecCommand(json, "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		data, err := fetchPRData("o/r", "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Expected order: Fail(beta), Pass(alpha), Pass(zebra)
		expected := []string{"beta", "alpha", "zebra"}
		for i, name := range expected {
			if data.Checks[i].Name != name {
				t.Errorf("checks[%d].Name = %q, want %q", i, data.Checks[i].Name, name)
			}
		}
	})

	t.Run("gh CLI error", func(t *testing.T) {
		execCommand = fakeExecCommand("", "not found", 1)
		t.Cleanup(func() { execCommand = exec.Command })

		_, err := fetchPRData("o/r", "1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, should contain stderr message", err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		execCommand = fakeExecCommand("not json", "", 0)
		t.Cleanup(func() { execCommand = exec.Command })

		_, err := fetchPRData("o/r", "1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse") {
			t.Errorf("error = %q, should contain 'failed to parse'", err)
		}
	})
}

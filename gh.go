package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// CheckStatus represents the normalized status of a check.
// The iota ordering matches the desired sort order.
type CheckStatus int

const (
	Running CheckStatus = iota
	Fail
	Pass
	Skipped
)

func (s CheckStatus) String() string {
	switch s {
	case Running:
		return "RUNNING"
	case Fail:
		return "FAIL"
	case Pass:
		return "PASS"
	case Skipped:
		return "SKIPPED"
	}
	return "UNKNOWN"
}

type Check struct {
	Name       string
	Status     CheckStatus
	Duration   string
	DetailsURL string
	StartedAt  time.Time
	Completed  bool
}

type PRData struct {
	Title       string
	HeadRefName string
	URL         string
	Checks      []Check
}

type ghPRResponse struct {
	Title              string        `json:"title"`
	HeadRefName        string        `json:"headRefName"`
	URL                string        `json:"url"`
	StatusCheckRollup  []ghCheckItem `json:"statusCheckRollup"`
}

type ghCheckItem struct {
	Typename     string `json:"__typename"`
	Name         string `json:"name"`
	Context      string `json:"context"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	StartedAt    string `json:"startedAt"`
	CompletedAt  string `json:"completedAt"`
	DetailsURL   string `json:"detailsUrl"`
	TargetURL    string `json:"targetUrl"`
	WorkflowName string `json:"workflowName"`
}

func normalizeStatus(raw string) CheckStatus {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	switch raw {
	case "SUCCESS", "PASS":
		return Pass
	case "FAILURE", "FAIL", "ERROR", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return Fail
	case "IN_PROGRESS", "RUNNING", "PENDING", "QUEUED", "WAITING", "REQUESTED":
		return Running
	case "SKIPPED", "CANCELLED", "NEUTRAL", "STALE":
		return Skipped
	case "":
		return Running
	}
	return Running
}

func parseDuration(startedAt string, completedAt string) (string, time.Time, bool) {
	if startedAt == "" {
		return "-", time.Time{}, false
	}
	start, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return "-", time.Time{}, false
	}

	completed := false
	var end time.Time
	if completedAt != "" {
		end, err = time.Parse(time.RFC3339, completedAt)
		if err == nil {
			completed = true
		}
	}
	if !completed {
		end = time.Now().UTC()
	}

	delta := int(end.Sub(start).Seconds())
	if delta < 0 {
		delta = 0
	}
	minutes := delta / 60
	seconds := delta % 60
	var dur string
	if minutes > 0 {
		dur = fmt.Sprintf("%dm%02ds", minutes, seconds)
	} else {
		dur = fmt.Sprintf("%ds", seconds)
	}
	return dur, start, completed
}

func fetchPRData(repo string, prNumber string) (*PRData, error) {
	cmd := exec.Command("gh", "pr", "view", prNumber,
		"--repo", repo,
		"--json", "statusCheckRollup,title,headRefName,url",
	)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh CLI error: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("gh CLI error: %w", err)
	}

	var resp ghPRResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	checks := make([]Check, 0, len(resp.StatusCheckRollup))
	for _, item := range resp.StatusCheckRollup {
		name := item.Name
		if name == "" {
			name = item.Context
		}
		if name == "" {
			name = "unknown"
		}
		if item.WorkflowName != "" {
			name = fmt.Sprintf("%s (%s)", name, item.WorkflowName)
		}

		var status CheckStatus
		if item.Conclusion != "" {
			status = normalizeStatus(item.Conclusion)
		} else {
			status = normalizeStatus(item.Status)
		}

		completedAt := item.CompletedAt
		if strings.HasPrefix(completedAt, "0001") {
			completedAt = ""
		}

		dur, startedAt, completed := parseDuration(item.StartedAt, completedAt)

		detailsURL := item.DetailsURL
		if detailsURL == "" {
			detailsURL = item.TargetURL
		}

		checks = append(checks, Check{
			Name:       name,
			Status:     status,
			Duration:   dur,
			DetailsURL: detailsURL,
			StartedAt:  startedAt,
			Completed:  completed,
		})
	}

	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Status != checks[j].Status {
			return checks[i].Status < checks[j].Status
		}
		return checks[i].Name < checks[j].Name
	})

	return &PRData{
		Title:       resp.Title,
		HeadRefName: resp.HeadRefName,
		URL:         resp.URL,
		Checks:      checks,
	}, nil
}

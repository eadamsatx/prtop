package main

import "testing"

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantRepo   string
		wantPR     string
		wantOK     bool
	}{
		{
			name:     "valid URL",
			url:      "https://github.com/owner/repo/pull/123",
			wantRepo: "owner/repo",
			wantPR:   "123",
			wantOK:   true,
		},
		{
			name:     "trailing slash",
			url:      "https://github.com/owner/repo/pull/123/",
			wantRepo: "owner/repo",
			wantPR:   "123",
			wantOK:   true,
		},
		{
			name:   "not github",
			url:    "https://gitlab.com/o/r/pull/1",
			wantOK: false,
		},
		{
			name:   "missing segments",
			url:    "https://github.com/owner/repo",
			wantOK: false,
		},
		{
			name:   "wrong segment",
			url:    "https://github.com/o/r/issues/1",
			wantOK: false,
		},
		{
			name:   "empty string",
			url:    "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pr, ok := parsePRURL(tt.url)
			if ok != tt.wantOK {
				t.Fatalf("parsePRURL(%q) ok = %v, want %v", tt.url, ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if pr != tt.wantPR {
				t.Errorf("prNumber = %q, want %q", pr, tt.wantPR)
			}
		})
	}
}

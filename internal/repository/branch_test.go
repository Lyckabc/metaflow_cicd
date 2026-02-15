package repository

import (
	"testing"

	"github.com/lib/pq"
)

func TestBranchMatchesTargetBranches(t *testing.T) {
	tests := []struct {
		branch   string
		patterns pq.StringArray
		want     bool
	}{
		{"main", pq.StringArray{"main"}, true},
		{"main", pq.StringArray{"main", "dev"}, true},
		{"dev", pq.StringArray{"main", "dev"}, true},
		{"feature/foo", pq.StringArray{"^feature/.*"}, true},
		{"feature/bar", pq.StringArray{"main", "^feature/.*"}, true},
		{"release-v1", pq.StringArray{"^release-v.*"}, true},
		{"other", pq.StringArray{"main", "dev"}, false},
		{"feature", pq.StringArray{"^feature/.*"}, false},
		{"", pq.StringArray{"main"}, false},
	}
	for _, tt := range tests {
		got := BranchMatchesTargetBranches(tt.branch, tt.patterns)
		if got != tt.want {
			t.Errorf("BranchMatchesTargetBranches(%q, %v) = %v, want %v", tt.branch, tt.patterns, got, tt.want)
		}
	}
}

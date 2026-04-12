package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSaveVerifyResultWritesLatestAndHistory(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	result := VerifyResult{
		Loop:             2,
		Passed:           false,
		Summary:          "smoke failed",
		ChecksRun:        []string{"go test ./..."},
		Failures:         []string{"GET /health returned 500"},
		PassedFeatureIDs: []string{},
	}
	if err := store.SaveVerifyResult("issue-1", result); err != nil {
		t.Fatalf("SaveVerifyResult: %v", err)
	}

	latest, err := LoadVerifyResult(store.IssueDir("issue-1"))
	if err != nil {
		t.Fatalf("LoadVerifyResult: %v", err)
	}
	if latest.Loop != 2 || latest.Summary != "smoke failed" {
		t.Fatalf("unexpected latest verify result: %+v", latest)
	}

	historyPath := VerifyHistoryPath(store.IssueDir("issue-1"), 2)
	if _, err := os.Stat(historyPath); err != nil {
		t.Fatalf("expected history file at %s: %v", historyPath, err)
	}
}

func TestLoadVerifyHistoryReturnsSortedResults(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	for _, result := range []VerifyResult{
		{Loop: 2, Passed: true, Summary: "second", ChecksRun: []string{"b"}, Failures: []string{}, PassedFeatureIDs: []string{"F-2"}},
		{Loop: 1, Passed: false, Summary: "first", ChecksRun: []string{"a"}, Failures: []string{"broken"}, PassedFeatureIDs: []string{}},
	} {
		if err := store.SaveVerifyResult("issue-1", result); err != nil {
			t.Fatalf("SaveVerifyResult(loop=%d): %v", result.Loop, err)
		}
	}

	history, err := LoadVerifyHistory(store.IssueDir("issue-1"))
	if err != nil {
		t.Fatalf("LoadVerifyHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].Loop != 1 || history[1].Loop != 2 {
		t.Fatalf("expected history sorted by loop, got %+v", history)
	}
}

func TestVerifyHistoryPathUsesLoopNaming(t *testing.T) {
	path := VerifyHistoryPath("/tmp/artifact", 7)
	if !strings.Contains(filepath.ToSlash(path), "verify_results/loop-07.json") {
		t.Fatalf("unexpected verify history path: %s", path)
	}
}

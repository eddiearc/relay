package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFeatureTransitionRejectsDeletionAndRegression(t *testing.T) {
	previous := []FeatureItem{
		{ID: "F-1", Title: "one", Description: "desc", Priority: 1, Passes: true},
		{ID: "F-2", Title: "two", Description: "desc", Priority: 2, Passes: false},
	}

	if err := ValidateFeatureTransition(previous, []FeatureItem{
		{ID: "F-1", Title: "one", Description: "desc", Priority: 1, Passes: true},
	}); err == nil {
		t.Fatalf("expected removal to fail")
	}

	if err := ValidateFeatureTransition(previous, []FeatureItem{
		{ID: "F-1", Title: "one", Description: "desc", Priority: 1, Passes: false},
		{ID: "F-2", Title: "two", Description: "desc", Priority: 2, Passes: false},
	}); err == nil {
		t.Fatalf("expected pass regression to fail")
	}

	if err := ValidateFeatureTransition(previous, []FeatureItem{
		{ID: "F-1", Title: "one", Description: "desc", Priority: 1, Passes: true},
		{ID: "F-2", Title: "two", Description: "desc", Priority: 5, Passes: false},
		{ID: "F-3", Title: "three", Description: "desc", Priority: 3, Passes: false},
	}); err != nil {
		t.Fatalf("expected extension to succeed: %v", err)
	}
}

func TestLoadFeatureListFixtures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		fixture string
		wantErr string
	}{
		{name: "valid", fixture: "feature_list_valid.json"},
		{name: "missing-title", fixture: "feature_list_invalid_missing_title.json", wantErr: `feature "verification-guide" is missing title`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			artifactDir := t.TempDir()
			fixturePath := filepath.Join("testdata", tc.fixture)
			data, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture %s: %v", fixturePath, err)
			}
			if err := os.WriteFile(FeatureListPath(artifactDir), data, 0o644); err != nil {
				t.Fatalf("write feature_list.json: %v", err)
			}

			items, err := LoadFeatureList(artifactDir)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("LoadFeatureList: %v", err)
				}
				if len(items) != 2 {
					t.Fatalf("expected 2 feature items, got %d", len(items))
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

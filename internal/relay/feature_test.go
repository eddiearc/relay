package relay

import "testing"

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

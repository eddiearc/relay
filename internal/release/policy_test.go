package release

import "testing"

func TestEvaluateNoopWhenLatestPublishedCoversHead(t *testing.T) {
	decision, err := Evaluate(PolicyInput{
		LatestPublishedTag:   "v1.2.3",
		LatestPublishedSHA:   "head",
		HeadSHA:              "head",
		UnreleasedCommits:    0,
		CoveringExplicitTags: nil,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Action != ActionNoop {
		t.Fatalf("expected %q, got %q", ActionNoop, decision.Action)
	}
	if decision.Tag != "" {
		t.Fatalf("expected empty tag, got %q", decision.Tag)
	}
	if decision.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestEvaluatePublishesExplicitCoveringTagFirst(t *testing.T) {
	decision, err := Evaluate(PolicyInput{
		LatestPublishedTag:   "v1.2.3",
		LatestPublishedSHA:   "old",
		HeadSHA:              "head",
		UnreleasedCommits:    2,
		CoveringExplicitTags: []string{"v1.3.0", "v1.2.4"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Action != ActionPublishExplicitTag {
		t.Fatalf("expected %q, got %q", ActionPublishExplicitTag, decision.Action)
	}
	if decision.Tag != "v1.3.0" {
		t.Fatalf("expected highest explicit tag, got %q", decision.Tag)
	}
}

func TestEvaluateFallsBackToNextPatch(t *testing.T) {
	decision, err := Evaluate(PolicyInput{
		LatestPublishedTag: "v1.2.3",
		LatestPublishedSHA: "old",
		HeadSHA:            "head",
		UnreleasedCommits:  4,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Action != ActionAutoCutPatch {
		t.Fatalf("expected %q, got %q", ActionAutoCutPatch, decision.Action)
	}
	if decision.Tag != "v1.2.4" {
		t.Fatalf("expected next patch tag, got %q", decision.Tag)
	}
}

func TestEvaluateRequiresExplicitFirstReleaseWhenNoPublishedBaselineExists(t *testing.T) {
	decision, err := Evaluate(PolicyInput{
		HeadSHA:           "head",
		UnreleasedCommits: 3,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Action != ActionNoop {
		t.Fatalf("expected %q, got %q", ActionNoop, decision.Action)
	}
	if decision.Tag != "" {
		t.Fatalf("expected empty tag, got %q", decision.Tag)
	}
}

func TestEvaluateAvoidsDuplicateReleaseWhenPublishedAlreadyCoversHead(t *testing.T) {
	decision, err := Evaluate(PolicyInput{
		LatestPublishedTag:   "v1.2.4",
		LatestPublishedSHA:   "head",
		HeadSHA:              "head",
		UnreleasedCommits:    0,
		CoveringExplicitTags: []string{"v1.3.0"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if decision.Action != ActionNoop {
		t.Fatalf("expected %q, got %q", ActionNoop, decision.Action)
	}
	if decision.Tag != "" {
		t.Fatalf("expected empty tag, got %q", decision.Tag)
	}
}

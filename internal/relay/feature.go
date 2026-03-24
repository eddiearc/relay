package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func FeatureListPath(repoPath string) string {
	return filepath.Join(repoPath, "feature_list.json")
}

func ProgressPath(artifactDir string) string {
	return filepath.Join(artifactDir, "progress.txt")
}

func LoadFeatureList(artifactDir string) ([]FeatureItem, error) {
	data, err := os.ReadFile(FeatureListPath(artifactDir))
	if err != nil {
		return nil, fmt.Errorf("read feature_list.json: %w", err)
	}
	var items []FeatureItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse feature_list.json: %w", err)
	}
	if err := ValidateFeatureList(items); err != nil {
		return nil, err
	}
	return items, nil
}

func ValidateFeatureList(items []FeatureItem) error {
	if len(items) == 0 {
		return errors.New("feature_list.json must contain at least one feature")
	}
	seen := map[string]struct{}{}
	for i, item := range items {
		if item.ID == "" {
			return fmt.Errorf("feature %d is missing id", i)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("feature id %q is duplicated", item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Title == "" {
			return fmt.Errorf("feature %q is missing title", item.ID)
		}
		if item.Description == "" {
			return fmt.Errorf("feature %q is missing description", item.ID)
		}
		if item.Priority <= 0 {
			return fmt.Errorf("feature %q has invalid priority %d", item.ID, item.Priority)
		}
	}
	return nil
}

func ValidateFeatureTransition(previous, current []FeatureItem) error {
	prevByID := map[string]FeatureItem{}
	for _, item := range previous {
		prevByID[item.ID] = item
	}
	currentByID := map[string]FeatureItem{}
	for _, item := range current {
		currentByID[item.ID] = item
	}
	for id, prev := range prevByID {
		next, ok := currentByID[id]
		if !ok {
			return fmt.Errorf("feature %q was removed", id)
		}
		if prev.Passes && !next.Passes {
			return fmt.Errorf("feature %q reverted passes from true to false", id)
		}
	}
	return nil
}

func AllFeaturesPassed(items []FeatureItem) bool {
	for _, item := range items {
		if !item.Passes {
			return false
		}
	}
	return true
}

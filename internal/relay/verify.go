package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type VerifyResult struct {
	Loop             int      `json:"loop"`
	Passed           bool     `json:"passed"`
	Summary          string   `json:"summary"`
	ChecksRun        []string `json:"checks_run"`
	Failures         []string `json:"failures"`
	PassedFeatureIDs []string `json:"passed_feature_ids"`
}

func VerifyResultPath(artifactDir string) string {
	return filepath.Join(artifactDir, "verify_result.json")
}

func VerifyHistoryDirPath(artifactDir string) string {
	return filepath.Join(artifactDir, "verify_results")
}

func VerifyHistoryPath(artifactDir string, loop int) string {
	return filepath.Join(VerifyHistoryDirPath(artifactDir), fmt.Sprintf("loop-%02d.json", loop))
}

func LoadVerifyResult(artifactDir string) (VerifyResult, error) {
	var result VerifyResult
	data, err := os.ReadFile(VerifyResultPath(artifactDir))
	if err != nil {
		return result, fmt.Errorf("read verify_result.json: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("parse verify_result.json: %w", err)
	}
	if err := ValidateVerifyResult(result); err != nil {
		return result, err
	}
	return result, nil
}

func LoadVerifyHistory(artifactDir string) ([]VerifyResult, error) {
	entries, err := os.ReadDir(VerifyHistoryDirPath(artifactDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read verify_results dir: %w", err)
	}
	results := make([]VerifyResult, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(VerifyHistoryDirPath(artifactDir), entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		var result VerifyResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		if err := ValidateVerifyResult(result); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Loop < results[j].Loop
	})
	return results, nil
}

func ValidateVerifyResult(result VerifyResult) error {
	if result.Loop <= 0 {
		return fmt.Errorf("verify_result.json loop must be positive, got %d", result.Loop)
	}
	if result.Summary == "" {
		return fmt.Errorf("verify_result.json summary is required")
	}
	if len(result.ChecksRun) == 0 {
		return fmt.Errorf("verify_result.json checks_run must contain at least one entry")
	}
	if result.Failures == nil {
		return fmt.Errorf("verify_result.json failures must be an array")
	}
	if result.PassedFeatureIDs == nil {
		return fmt.Errorf("verify_result.json passed_feature_ids must be an array")
	}
	if result.Passed {
		if len(result.Failures) != 0 {
			return fmt.Errorf("verify_result.json failures must be empty when passed=true")
		}
		if len(result.PassedFeatureIDs) == 0 {
			return fmt.Errorf("verify_result.json passed_feature_ids must contain at least one feature when passed=true")
		}
	}
	return nil
}

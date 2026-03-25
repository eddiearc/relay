package relay

import (
	"bytes"
	"fmt"
	"os"

	"encoding/json"

	"gopkg.in/yaml.v3"
)

func LoadPipeline(path string) (Pipeline, error) {
	var pipeline Pipeline
	data, err := os.ReadFile(path)
	if err != nil {
		return pipeline, fmt.Errorf("read pipeline: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&pipeline); err != nil {
		return pipeline, fmt.Errorf("parse pipeline: %w", err)
	}
	if err := pipeline.Normalize(); err != nil {
		return pipeline, err
	}
	return pipeline, nil
}

func LoadIssue(path string) (Issue, error) {
	var issue Issue
	data, err := os.ReadFile(path)
	if err != nil {
		return issue, fmt.Errorf("read issue: %w", err)
	}
	if err := json.Unmarshal(data, &issue); err != nil {
		return issue, fmt.Errorf("parse issue: %w", err)
	}
	if err := issue.Normalize(); err != nil {
		return issue, err
	}
	return issue, nil
}

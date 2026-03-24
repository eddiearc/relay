package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

const workspaceRootEnvVar = "RELAY_WORKSPACE_ROOT"

type Store struct {
	Root          string
	WorkspaceRoot string
}

func NewStore(root string) *Store {
	return &Store{
		Root:          root,
		WorkspaceRoot: DefaultWorkspaceRoot(),
	}
}

func DefaultWorkspaceRoot() string {
	if value := os.Getenv(workspaceRootEnvVar); value != "" {
		return value
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "relay-workspaces")
	}
	return "relay-workspaces"
}

func (s *Store) Ensure() error {
	for _, dir := range []string{s.Root, s.IssuesDir(), s.PipelinesDir(), s.WorkspaceRoot} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) IssuesDir() string {
	return filepath.Join(s.Root, "issues")
}

func (s *Store) PipelinesDir() string {
	return filepath.Join(s.Root, "pipelines")
}

func (s *Store) IssueDir(issueID string) string {
	return filepath.Join(s.IssuesDir(), issueID)
}

func IssueFilePath(issueDir string) string {
	return filepath.Join(issueDir, "issue.json")
}

func (s *Store) IssuePath(issueID string) string {
	return IssueFilePath(s.IssueDir(issueID))
}

func (s *Store) RunDir(issueID string) string {
	return filepath.Join(s.IssueDir(issueID), "runs")
}

func (s *Store) EventsPath(issueID string) string {
	return filepath.Join(s.IssueDir(issueID), "events.log")
}

func (s *Store) PipelinePath(name string) string {
	return filepath.Join(s.PipelinesDir(), name+".yaml")
}

func (s *Store) SaveIssue(issue Issue) error {
	issue.ArtifactDir = s.IssueDir(issue.ID)
	data, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal issue: %w", err)
	}
	if err := os.MkdirAll(issue.ArtifactDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(IssueFilePath(issue.ArtifactDir), data, 0o644)
}

func (s *Store) LoadIssue(issueID string) (Issue, error) {
	var issue Issue
	data, err := os.ReadFile(s.IssuePath(issueID))
	if err != nil {
		return issue, err
	}
	if err := json.Unmarshal(data, &issue); err != nil {
		return issue, fmt.Errorf("parse issue.json: %w", err)
	}
	if issue.ArtifactDir == "" {
		issue.ArtifactDir = s.IssueDir(issue.ID)
	}
	return issue, nil
}

func (s *Store) ListIssues() ([]Issue, error) {
	entries, err := os.ReadDir(s.IssuesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	issues := make([]Issue, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		issue, err := s.LoadIssue(entry.Name())
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].ID < issues[j].ID
	})
	return issues, nil
}

func (s *Store) SavePipeline(pipeline Pipeline) error {
	if err := pipeline.Normalize(); err != nil {
		return err
	}
	data, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("marshal pipeline: %w", err)
	}
	if err := os.MkdirAll(s.PipelinesDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.PipelinePath(pipeline.Name), data, 0o644)
}

func (s *Store) LoadPipeline(name string) (Pipeline, error) {
	return LoadPipeline(s.PipelinePath(name))
}

func (s *Store) ListPipelines() ([]Pipeline, error) {
	entries, err := os.ReadDir(s.PipelinesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	pipelines := make([]Pipeline, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		pipeline, err := s.LoadPipeline(entry.Name()[:len(entry.Name())-len(".yaml")])
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, pipeline)
	}
	sort.Slice(pipelines, func(i, j int) bool {
		return pipelines[i].Name < pipelines[j].Name
	})
	return pipelines, nil
}

func (s *Store) SaveRunLog(issueID, name, stdout, stderr, finalMessage string) error {
	runDir := s.RunDir(issueID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, name+".stdout.log"), []byte(stdout), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, name+".stderr.log"), []byte(stderr), 0o644); err != nil {
		return err
	}
	if finalMessage != "" {
		if err := os.WriteFile(filepath.Join(runDir, name+".final.txt"), []byte(finalMessage), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) AppendEvent(issueID, message string) error {
	issueDir := s.IssueDir(issueID)
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(s.EventsPath(issueID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	line := fmt.Sprintf("%s %s\n", time.Now().UTC().Format(time.RFC3339), message)
	_, err = file.WriteString(line)
	return err
}

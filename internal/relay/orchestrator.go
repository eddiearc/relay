package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
)

type Orchestrator struct {
	Store  *Store
	Shell  ShellRunner
	Runner AgentRunner

	mu      sync.Mutex
	running map[string]*RunningTask
}

type RunningTask struct {
	IssueID       string
	PipelineName  string
	Phase         string
	CurrentLoop   int
	WorkspacePath string
	RepoPath      string
	ActivePIDs    []int
}

func NewOrchestrator(store *Store, shell ShellRunner, runner AgentRunner) *Orchestrator {
	return &Orchestrator{
		Store:  store,
		Shell:  shell,
		Runner: runner,
		running: map[string]*RunningTask{},
	}
}

func (o *Orchestrator) RunIssue(ctx context.Context, pipeline Pipeline, issue Issue) (Issue, error) {
	issue.Status = IssueStatusRunning
	issue.ArtifactDir = o.Store.IssueDir(issue.ID)
	if err := o.Store.Ensure(); err != nil {
		return issue, err
	}
	if err := o.setIssuePhase(&issue, "init", true); err != nil {
		return issue, err
	}
	_ = o.Store.AppendEvent(issue.ID, "issue started")

	workspacePath, err := o.createWorkspace(issue.ID)
	if err != nil {
		issue.LastError = err.Error()
		issue.Status = IssueStatusFailed
		_ = o.Store.SaveIssue(issue)
		return issue, err
	}
	issue.WorkspacePath = workspacePath
	if err := o.Store.SaveIssue(issue); err != nil {
		return issue, err
	}
	_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("workspace created path=%s", workspacePath))

	_ = o.Store.AppendEvent(issue.ID, "init_command started")
	initResult, err := o.Shell.Run(ctx, ShellRunRequest{
		Workdir: workspacePath,
		Command: pipeline.InitCommand,
		OnStart: func(pid int) {
			o.trackIssuePID(&issue, "init", pid)
		},
	})
	if saveErr := o.Store.SaveRunLog(issue.ID, "init", initResult.Stdout, initResult.Stderr, ""); saveErr != nil {
		return issue, saveErr
	}
	if err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("init_command failed: %v", err))
		return o.failIssue(issue, err)
	}
	_ = o.Store.AppendEvent(issue.ID, "init_command completed")
	if latest, stopped, err := o.finalizeExternalState(issue.ID); err != nil {
		return o.failIssue(issue, err)
	} else if stopped {
		return latest, nil
	} else {
		issue = latest
	}

	repoPath, err := DiscoverRepoRoot(ctx, workspacePath)
	if err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("repo discovery failed: %v", err))
		return o.failIssue(issue, err)
	}
	issue.RepoPath = repoPath
	if err := o.Store.SaveIssue(issue); err != nil {
		return issue, err
	}
	_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("repo discovered path=%s", repoPath))

	issue.Status = IssueStatusPlanning
	if err := o.setIssuePhase(&issue, "plan", true); err != nil {
		return issue, err
	}
	if err := o.runPlanning(ctx, pipeline, &issue); err != nil {
		if latest, stopped, stopErr := o.finalizeExternalState(issue.ID); stopErr != nil {
			return o.failIssue(issue, stopErr)
		} else if stopped {
			return latest, nil
		}
		return o.failIssue(issue, err)
	}
	if latest, stopped, err := o.finalizeExternalState(issue.ID); err != nil {
		return o.failIssue(issue, err)
	} else if stopped {
		return latest, nil
	} else {
		issue = latest
	}

	for loop := 1; loop <= pipeline.LoopNum; loop++ {
		issue.Status = IssueStatusRunning
		issue.CurrentLoop = loop
		if err := o.setIssuePhase(&issue, "coding", true); err != nil {
			return issue, err
		}
		done, err := o.runCodingLoop(ctx, pipeline, &issue, loop)
		if err != nil {
			issue.LastError = err.Error()
			if saveErr := o.Store.SaveIssue(issue); saveErr != nil {
				return issue, saveErr
			}
		}
		if latest, stopped, err := o.finalizeExternalState(issue.ID); err != nil {
			return o.failIssue(issue, err)
		} else if stopped {
			return latest, nil
		} else {
			issue = latest
		}
		if err != nil {
			_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d aborted; advancing to next loop", loop))
			continue
		}
		if done {
			issue.Status = IssueStatusDone
			issue.LastError = ""
			if err := o.clearIssueRuntime(&issue); err != nil {
				return issue, err
			}
			_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("issue completed loop=%d", loop))
			return issue, nil
		}
	}

	return o.failIssue(issue, fmt.Errorf("loop limit reached after %d iterations", pipeline.LoopNum))
}

func (o *Orchestrator) runPlanning(ctx context.Context, pipeline Pipeline, issue *Issue) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
			_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("planning failed: %v\n%s", err, strings.TrimSpace(string(debug.Stack()))))
		}
	}()
	_ = o.Store.AppendEvent(issue.ID, "planning started")
	prompt := BuildPrompt(*issue, "plan", 0, pipeline.PlanPrompt)
	result, err := o.Runner.Run(ctx, AgentRunRequest{
		Phase:    "plan",
		RepoPath: issue.RepoPath,
		Prompt:   prompt,
		IssueID:  issue.ID,
		LoopID:   "plan",
		OnPID: func(pid int) {
			o.trackIssuePID(issue, "plan", pid)
		},
	})
	if saveErr := o.Store.SaveRunLog(issue.ID, "plan", result.Stdout, result.Stderr, result.FinalMessage); saveErr != nil {
		return saveErr
	}
	if err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("planning failed: %v", err))
		return err
	}
	if _, statErr := os.Stat(ProgressPath(issue.ArtifactDir)); statErr != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("planning validation failed: %v", statErr))
		return fmt.Errorf("planning did not create progress.txt: %w", statErr)
	}
	items, err := LoadFeatureList(issue.ArtifactDir)
	if err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("planning validation failed: %v", err))
		return err
	}
	if len(items) == 0 {
		_ = o.Store.AppendEvent(issue.ID, "planning validation failed: feature_list.json empty")
		return errors.New("planning produced an empty feature_list.json")
	}
	issue.LastError = ""
	_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("planning completed features=%d", len(items)))
	return nil
}

func (o *Orchestrator) runCodingLoop(ctx context.Context, pipeline Pipeline, issue *Issue, loop int) (_ bool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
			_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d failed: %v\n%s", loop, err, strings.TrimSpace(string(debug.Stack()))))
		}
	}()
	beforeItems, err := LoadFeatureList(issue.ArtifactDir)
	if err != nil {
		return false, err
	}
	beforeRev, err := gitRevision(issue.RepoPath)
	if err != nil {
		return false, fmt.Errorf("coding pre-check git revision: %w", err)
	}
	_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d started", loop))
	prompt := BuildPrompt(*issue, "coding", loop, pipeline.CodingPrompt) + TailContext(issue.ArtifactDir)
	loopID := fmt.Sprintf("loop-%02d", loop)
	result, err := o.Runner.Run(ctx, AgentRunRequest{
		Phase:    "coding",
		RepoPath: issue.RepoPath,
		Prompt:   prompt,
		IssueID:  issue.ID,
		LoopID:   loopID,
		OnPID: func(pid int) {
			o.trackIssuePID(issue, "coding", pid)
		},
	})
	if saveErr := o.Store.SaveRunLog(issue.ID, loopID, result.Stdout, result.Stderr, result.FinalMessage); saveErr != nil {
		return false, saveErr
	}
	if err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d failed: %v", loop, err))
		return false, err
	}
	if _, statErr := os.Stat(ProgressPath(issue.ArtifactDir)); statErr != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d validation failed: %v", loop, statErr))
		return false, fmt.Errorf("coding loop %d is missing progress.txt: %w", loop, statErr)
	}
	afterItems, err := LoadFeatureList(issue.ArtifactDir)
	if err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d validation failed: %v", loop, err))
		return false, err
	}
	if err := ValidateFeatureTransition(beforeItems, afterItems); err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d validation failed: %v", loop, err))
		return false, err
	}
	if err := ensureGitRevisionChanged(issue.RepoPath, beforeRev); err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d validation failed: %v", loop, err))
		return false, fmt.Errorf("coding loop %d must create a git commit: %w", loop, err)
	}
	done := AllFeaturesPassed(afterItems)
	issue.LastError = ""
	_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("coding loop=%d completed done=%t", loop, done))
	return done, nil
}

func (o *Orchestrator) createWorkspace(issueID string) (string, error) {
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	path := filepath.Join(o.Store.WorkspaceRoot, issueID+"-"+hex.EncodeToString(suffix))
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func (o *Orchestrator) failIssue(issue Issue, err error) (Issue, error) {
	issue.Status = IssueStatusFailed
	issue.LastError = err.Error()
	_ = o.clearIssueRuntime(&issue)
	_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("issue failed: %v", err))
	return issue, err
}

func (o *Orchestrator) finalizeExternalState(issueID string) (Issue, bool, error) {
	latest, err := o.Store.LoadIssue(issueID)
	if err != nil {
		return Issue{}, false, err
	}
	if latest.Status == IssueStatusInterrupted {
		if err := o.clearIssueRuntime(&latest); err != nil {
			return Issue{}, false, err
		}
		return latest, true, nil
	}
	if latest.Status == IssueStatusFailed || latest.Status == IssueStatusDeleted {
		if err := o.clearIssueRuntime(&latest); err != nil {
			return Issue{}, false, err
		}
		_ = o.Store.AppendEvent(issueID, fmt.Sprintf("issue stopped with status=%s", latest.Status))
		return latest, true, nil
	}
	if !latest.InterruptRequested {
		return latest, false, nil
	}
	latest.Status = IssueStatusInterrupted
	latest.LastError = "interrupted by user"
	latest.InterruptRequested = false
	if err := o.clearIssueRuntime(&latest); err != nil {
		return Issue{}, false, err
	}
	_ = o.Store.AppendEvent(issueID, "interrupt request finalized")
	return latest, true, nil
}

func (o *Orchestrator) RunningIssue(issueID string) (RunningTask, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	task, ok := o.running[issueID]
	if !ok {
		return RunningTask{}, false
	}
	return copyRunningTask(task), true
}

func (o *Orchestrator) setIssuePhase(issue *Issue, phase string, resetPIDs bool) error {
	o.mu.Lock()
	task := o.running[issue.ID]
	if task == nil {
		task = &RunningTask{IssueID: issue.ID}
		o.running[issue.ID] = task
	}
	task.PipelineName = issue.PipelineName
	task.Phase = phase
	task.CurrentLoop = issue.CurrentLoop
	task.WorkspacePath = issue.WorkspacePath
	task.RepoPath = issue.RepoPath
	if resetPIDs {
		task.ActivePIDs = nil
	}
	issue.ActivePhase = task.Phase
	issue.ActivePIDs = append([]int(nil), task.ActivePIDs...)
	o.mu.Unlock()
	return o.Store.SaveIssue(*issue)
}

func (o *Orchestrator) trackIssuePID(issue *Issue, phase string, pid int) {
	if pid <= 0 {
		return
	}

	o.mu.Lock()
	task := o.running[issue.ID]
	if task == nil {
		task = &RunningTask{IssueID: issue.ID}
		o.running[issue.ID] = task
	}
	task.PipelineName = issue.PipelineName
	task.Phase = phase
	task.CurrentLoop = issue.CurrentLoop
	task.WorkspacePath = issue.WorkspacePath
	task.RepoPath = issue.RepoPath
	if !containsPID(task.ActivePIDs, pid) {
		task.ActivePIDs = append(task.ActivePIDs, pid)
	}
	issue.ActivePhase = task.Phase
	issue.ActivePIDs = append([]int(nil), task.ActivePIDs...)
	o.mu.Unlock()

	if err := o.Store.SaveIssue(*issue); err != nil {
		_ = o.Store.AppendEvent(issue.ID, fmt.Sprintf("runtime pid persistence failed: %v", err))
	}
}

func (o *Orchestrator) clearIssueRuntime(issue *Issue) error {
	o.mu.Lock()
	delete(o.running, issue.ID)
	o.mu.Unlock()

	issue.ActivePhase = ""
	issue.ActivePIDs = nil
	return o.Store.SaveIssue(*issue)
}

func copyRunningTask(task *RunningTask) RunningTask {
	if task == nil {
		return RunningTask{}
	}
	return RunningTask{
		IssueID:       task.IssueID,
		PipelineName:  task.PipelineName,
		Phase:         task.Phase,
		CurrentLoop:   task.CurrentLoop,
		WorkspacePath: task.WorkspacePath,
		RepoPath:      task.RepoPath,
		ActivePIDs:    append([]int(nil), task.ActivePIDs...),
	}
}

func containsPID(pids []int, pid int) bool {
	for _, existing := range pids {
		if existing == pid {
			return true
		}
	}
	return false
}

func DiscoverRepoRoot(ctx context.Context, workspacePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = workspacePath
	if output, err := cmd.Output(); err == nil {
		repoPath := strings.TrimSpace(string(output))
		if repoPath != "" {
			return repoPath, nil
		}
	}

	var candidates []string
	err := filepath.WalkDir(workspacePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Name() != ".git" {
			return nil
		}
		candidates = append(candidates, filepath.Dir(path))
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	switch len(candidates) {
	case 0:
		return "", errors.New("init_command did not create a git repository")
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("init_command created multiple repositories: %v", candidates)
	}
}

func gitRevision(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func ensureGitRevisionChanged(repoPath, before string) error {
	after, err := gitRevision(repoPath)
	if err != nil {
		return err
	}
	if after == "" {
		return errors.New("git HEAD is empty after agent run")
	}
	if after == before {
		return errors.New("git HEAD did not change")
	}
	return nil
}

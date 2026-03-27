package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectRepositoryFindsExplicitCoveringTagOnHead(t *testing.T) {
	repo := initTestRepo(t)
	commitFile(t, repo, "base.txt", "base\n")
	runGit(t, repo, "tag", "v1.2.3")
	commitFile(t, repo, "next.txt", "next\n")
	runGit(t, repo, "tag", "v1.3.0")

	ctx, err := InspectRepository(repo, InspectOptions{
		MainRef:              "HEAD",
		PublishedReleaseTags: []string{"v1.2.3"},
	})
	if err != nil {
		t.Fatalf("InspectRepository: %v", err)
	}
	if ctx.LatestPublishedTag != "v1.2.3" {
		t.Fatalf("expected latest published tag v1.2.3, got %q", ctx.LatestPublishedTag)
	}
	if ctx.UnreleasedCommits != 1 {
		t.Fatalf("expected 1 unreleased commit, got %d", ctx.UnreleasedCommits)
	}
	if len(ctx.CoveringExplicitTags) != 1 || ctx.CoveringExplicitTags[0] != "v1.3.0" {
		t.Fatalf("expected explicit head tag v1.3.0, got %#v", ctx.CoveringExplicitTags)
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Relay Test")
	runGit(t, repo, "config", "user.email", "relay@example.com")
	return repo
}

func commitFile(t *testing.T, repo, name, content string) {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runGit(t, repo, "add", name)
	runGit(t, repo, "commit", "-m", strings.TrimSuffix(name, filepath.Ext(name)))
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

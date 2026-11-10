package ops_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/git/ops"
)

func TestPullerMerge(t *testing.T) {
	remoteDir, runner := initTempRepo(t)
	localDir, _ := initTempRepo(t)

	cmd := exec.Command(runner.Executable, "remote", "add", "origin", "file://"+filepath.ToSlash(remoteDir))
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// We need to fetch the remote branch first and set upstream so pull knows what to merge
	cmd = exec.Command(runner.Executable, "fetch", "origin")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Reset local to remote master to avoid unrelated histories
	cmd = exec.Command(runner.Executable, "reset", "--hard", "origin/master")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(runner.Executable, "branch", "--set-upstream-to=origin/master", "master")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(filepath.Join(remoteDir, "test2.txt"), []byte("update"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(runner.Executable, "add", "test2.txt")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(runner.Executable, "commit", "-m", "update")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	puller := ops.NewPuller(runner)

	err = puller.Pull(ctx, ops.PullParams{
		RepoPath: localDir,
		Remote:   "origin",
		Branch:   "master",
		Method:   ops.PullMethodMerge,
	})
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	_, err = os.Stat(filepath.Join(localDir, "test2.txt"))
	if err != nil {
		t.Fatalf("test2.txt should have been pulled: %v", err)
	}
}

func TestPullerRebase(t *testing.T) {
	remoteDir, runner := initTempRepo(t)
	localDir, _ := initTempRepo(t)

	cmd := exec.Command(runner.Executable, "remote", "add", "origin", "file://"+filepath.ToSlash(remoteDir))
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(runner.Executable, "fetch", "origin")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(runner.Executable, "reset", "--hard", "origin/master")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(runner.Executable, "branch", "--set-upstream-to=origin/master", "master")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Update remote
	err := os.WriteFile(filepath.Join(remoteDir, "test2.txt"), []byte("remote"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(runner.Executable, "add", "test2.txt")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(runner.Executable, "commit", "-m", "remote")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Update local concurrently
	err = os.WriteFile(filepath.Join(localDir, "test3.txt"), []byte("local"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(runner.Executable, "add", "test3.txt")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(runner.Executable, "commit", "-m", "local")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	puller := ops.NewPuller(runner)

	err = puller.Pull(ctx, ops.PullParams{
		RepoPath: localDir,
		Remote:   "origin",
		Branch:   "master",
		Method:   ops.PullMethodRebase,
	})
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	cmd = exec.Command(runner.Executable, "log", "--oneline")
	cmd.Dir = localDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	log := string(out)

	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 commits, got: %s", log)
	}
	if !strings.Contains(lines[0], "local") {
		t.Errorf("Expected latest commit to be 'local', got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "remote") {
		t.Errorf("Expected parent commit to be 'remote', got: %s", lines[1])
	}
}

func TestPullerStash(t *testing.T) {
	_, runner := initTempRepo(t)
	localDir, _ := initTempRepo(t)

	// Make local uncommitted changes
	err := os.WriteFile(filepath.Join(localDir, "test.txt"), []byte("uncommitted"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	puller := ops.NewPuller(runner)

	err = puller.Stash(ctx, localDir, "saving changes")
	if err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	hasStash, err := puller.StashList(ctx, localDir)
	if err != nil {
		t.Fatalf("StashList failed: %v", err)
	}
	if !hasStash {
		t.Fatal("Expected stash list to not be empty")
	}

	content, err := os.ReadFile(filepath.Join(localDir, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "initial" {
		t.Errorf("Expected test.txt to be reset to 'initial', got: %s", string(content))
	}

	err = puller.Unstash(ctx, localDir)
	if err != nil {
		t.Fatalf("Unstash failed: %v", err)
	}

	content, err = os.ReadFile(filepath.Join(localDir, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "uncommitted" {
		t.Errorf("Expected test.txt to be restored to 'uncommitted', got: %s", string(content))
	}
}

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

func TestFetcher(t *testing.T) {
	remoteDir, runner := initTempRepo(t)
	localDir, _ := initTempRepo(t)

	cmd := exec.Command(runner.Executable, "remote", "add", "origin", "file://"+filepath.ToSlash(remoteDir))
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
	cmd = exec.Command(runner.Executable, "branch", "feature")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fetcher := ops.NewFetcher(runner)

	err = fetcher.Fetch(ctx, ops.FetchParams{
		RepoPath: localDir,
		Remotes:  []string{"origin"},
	})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	cmd = exec.Command(runner.Executable, "branch", "-r")
	cmd.Dir = localDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	branches := string(out)
	if !strings.Contains(branches, "origin/master") {
		t.Errorf("Expected origin/master, got: %s", branches)
	}
	if !strings.Contains(branches, "origin/feature") {
		t.Errorf("Expected origin/feature, got: %s", branches)
	}

	// Test prune
	cmd = exec.Command(runner.Executable, "branch", "-d", "feature")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	err = fetcher.Fetch(ctx, ops.FetchParams{
		RepoPath: localDir,
		Remotes:  []string{"origin"},
		Prune:    true,
	})
	if err != nil {
		t.Fatalf("Fetch with prune failed: %v", err)
	}

	cmd = exec.Command(runner.Executable, "branch", "-r")
	cmd.Dir = localDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	branches = string(out)
	if strings.Contains(branches, "origin/feature") {
		t.Errorf("Expected origin/feature to be pruned, got: %s", branches)
	}
}

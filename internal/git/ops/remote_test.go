package ops_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/git/ops"
)

func TestRemoteManager(t *testing.T) {
	localDir, runner := initTempRepo(t)
	rm := ops.New(runner, localDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test Add
	err := rm.AddRemote(ctx, "origin", "file:///tmp/repo")
	if err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}

	cmd := exec.Command(runner.Executable, "remote", "-v")
	cmd.Dir = localDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "origin\tfile:///tmp/repo") {
		t.Errorf("Expected origin to be added, got: %s", string(out))
	}

	// Test SetURL
	err = rm.SetRemoteURL(ctx, "origin", "file:///tmp/repo2")
	if err != nil {
		t.Fatalf("SetRemoteURL failed: %v", err)
	}

	cmd = exec.Command(runner.Executable, "remote", "-v")
	cmd.Dir = localDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "origin\tfile:///tmp/repo2") {
		t.Errorf("Expected origin url to be updated, got: %s", string(out))
	}

	// Test Rename
	err = rm.RenameRemote(ctx, "origin", "upstream")
	if err != nil {
		t.Fatalf("RenameRemote failed: %v", err)
	}

	cmd = exec.Command(runner.Executable, "remote", "-v")
	cmd.Dir = localDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "upstream\tfile:///tmp/repo2") {
		t.Errorf("Expected origin to be renamed to upstream, got: %s", string(out))
	}

	// Test Remove
	err = rm.RemoveRemote(ctx, "upstream")
	if err != nil {
		t.Fatalf("RemoveRemote failed: %v", err)
	}

	cmd = exec.Command(runner.Executable, "remote", "-v")
	cmd.Dir = localDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "upstream") {
		t.Errorf("Expected upstream to be removed, got: %s", string(out))
	}
}

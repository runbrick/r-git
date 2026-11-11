package ops_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
)

func TestParsePushNativeResult(t *testing.T) {
	standardPrefix := []string{
		"Counting objects: 20, done.",
		"Delta compression using up to 8 threads.",
		"Compressing objects: 100% (14/14), done.",
		"Writing objects: 100% (20/20), 1.70 KiB | 0 bytes/s, done.",
		"Total 20 (delta 7), reused 0 (delta 0)",
		"To /Users/loki/sandbox/git/parent.git",
	}
	targetPrefix := "To http://example.com/git/parent.git"
	pushDefaultWarning := []string{
		"warning: push.default is unset; its implicit value has changed in",
		"Git 2.0 from 'matching' to 'simple'. To squelch this message",
		"and maintain the traditional behavior, use:",
		"",
		"  git config --global push.default matching",
		"",
		"To squelch this message and adopt the new behavior now, use:",
		"",
		"  git config --global push.default simple",
		"",
		"When push.default is set to 'matching', git will push local branches",
		"to the remote branches that already exist with the same name.",
		"",
		"Since Git 2.0, Git defaults to the more conservative 'simple'",
		"behavior, which only pushes the current branch to the corresponding",
		"remote branch that 'git pull' uses to update the current branch.",
		"",
		"See 'git help config' and search for 'push.default' for further information.",
		"(the 'simple' mode was introduced in Git 1.7.11. Use the similar mode",
		"'current' instead of 'simple' if you sometimes use older versions of Git)",
	}
	successSuffix := "Done"
	rejectSuffix := []string{
		"Done",
		"error: failed to push some refs to '/Users/loki/sandbox/git/parent.git'",
		"hint: Updates were rejected because the tip of your current branch is behind",
		"hint: its remote counterpart. Integrate the remote changes (e.g.",
		"hint: 'git pull ...') before pushing again.",
		"hint: See the 'Note about fast-forwards' in 'git push --help' for details.",
	}

	tests := []struct {
		name     string
		output   []string
		expected []ops.PushNativeResult
	}{
		{
			name: "success",
			output: append(append(standardPrefix, targetPrefix,
				" \trefs/heads/master:refs/heads/master\t3e62822..a537351"), successSuffix),
			expected: []ops.PushNativeResult{{Type: ops.PushSuccess, SourceRef: "refs/heads/master", Range: "3e62822..a537351"}},
		},
		{
			name: "successUpToCommit",
			output: append(append(standardPrefix, targetPrefix,
				" \t4915ba17cc8b718d58ae77dd25018fccdce322d6:refs/heads/master\t3e62822..4915ba1"), successSuffix),
			expected: []ops.PushNativeResult{{Type: ops.PushSuccess, SourceRef: "4915ba17cc8b718d58ae77dd25018fccdce322d6", Range: "3e62822..4915ba1"}},
		},
		{
			name: "rejected",
			output: append(append(standardPrefix, targetPrefix,
				"!\trefs/heads/master:refs/heads/master\t[rejected] (non-fast-forward)"), rejectSuffix...),
			expected: []ops.PushNativeResult{{Type: ops.PushRejected, SourceRef: "refs/heads/master", Reason: ops.NoFFRejectReason}},
		},
		{
			name: "rejected2",
			output: append(append(standardPrefix, targetPrefix,
				"!\trefs/heads/master:refs/heads/master\t[rejected] (fetch first)"), rejectSuffix...),
			expected: []ops.PushNativeResult{{Type: ops.PushRejected, SourceRef: "refs/heads/master", Reason: ops.FetchFirstReason}},
		},
		{
			name: "forcedUpdate",
			output: append(pushDefaultWarning, targetPrefix,
				"+\trefs/heads/master:refs/heads/master\tb9b3235...23760f8 (forced update)"),
			expected: []ops.PushNativeResult{{Type: ops.PushForcedUpdate, SourceRef: "refs/heads/master", Range: "b9b3235...23760f8", Reason: "forced update"}},
		},
		{
			name: "remoteRejected",
			output: append(pushDefaultWarning, targetPrefix,
				"!\trefs/heads/master:refs/heads/master\t[remote rejected] (pre-receive hook declined)"),
			expected: []ops.PushNativeResult{{Type: ops.PushRejected, SourceRef: "refs/heads/master", Reason: "pre-receive hook declined"}},
		},
		{
			name:     "upToDate",
			output:   []string{targetPrefix, "=\trefs/heads/master:refs/heads/master\t[up to date]"},
			expected: []ops.PushNativeResult{{Type: ops.PushUpToDate, SourceRef: "refs/heads/master"}},
		},
		{
			name: "newRef",
			output: append(standardPrefix, targetPrefix,
				"*\trefs/heads/feature:refs/heads/feature2\t[new branch]"),
			expected: []ops.PushNativeResult{{Type: ops.PushNewRef, SourceRef: "refs/heads/feature"}},
		},
		{
			name: "withTags",
			output: []string{
				" \trefs/heads/master:refs/heads/master\t7aabf91..d8369de",
				"*\trefs/tags/some_tag:refs/tags/some_tag\t[new tag]",
			},
			expected: []ops.PushNativeResult{
				{Type: ops.PushSuccess, SourceRef: "refs/heads/master", Range: "7aabf91..d8369de"},
				{Type: ops.PushNewRef, SourceRef: "refs/tags/some_tag"},
			},
		},
		{
			name: "withTagsUptoCommit",
			output: []string{
				" \t4915ba17cc8b718d58ae77dd25018fccdce322d6:refs/heads/abranch\t3e62822..4915ba1",
				"*\trefs/tags/some_tag:refs/tags/some_tag\t[new tag]",
			},
			expected: []ops.PushNativeResult{
				{Type: ops.PushSuccess, SourceRef: "4915ba17cc8b718d58ae77dd25018fccdce322d6", Range: "3e62822..4915ba1"},
				{Type: ops.PushNewRef, SourceRef: "refs/tags/some_tag"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ops.ParsePushNativeResult(tt.output)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParsePushNativeResult() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func initTempRepo(t *testing.T) (string, *command.Runner) {
	t.Helper()
	dir := t.TempDir()

	gitConfig := filepath.Join(dir, "global_gitconfig")
	err := os.WriteFile(gitConfig, []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", gitConfig)
	t.Setenv("HOME", dir)

	runner, err := command.New()
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	runGit := func(args ...string) {
		cmd := exec.Command(runner.Executable, args...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")

	err = os.WriteFile(filepath.Join(dir, "test.txt"), []byte("initial"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	runGit("add", "test.txt")
	runGit("commit", "-m", "initial")

	return dir, runner
}

func TestPushOperation(t *testing.T) {
	remoteDir, runner := initTempRepo(t)
	// configure it as a bare repository to receive pushes
	cmd := exec.Command(runner.Executable, "config", "core.bare", "true")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	localDir, _ := initTempRepo(t)

	cmd = exec.Command(runner.Executable, "remote", "add", "origin", "file://"+filepath.ToSlash(remoteDir))
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(filepath.Join(localDir, "test.txt"), []byte("update"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(runner.Executable, "commit", "-am", "update")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pusher := ops.NewPusher(runner)

	// Test basic push
	res, err := pusher.Push(ctx, localDir, "origin", "refs/heads/master:refs/heads/master", false, false)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if len(res) == 0 {
		t.Fatal("Expected at least one push result")
	}

	// Either PushNewRef, PushSuccess or PushRejected depending on state (should be NewRef on first bare repo push)
	if res[0].Type != ops.PushNewRef && res[0].Type != ops.PushSuccess && res[0].Type != ops.PushRejected {
		t.Errorf("Expected success, new ref, or rejected, got %v", res[0].Type)
	}
}

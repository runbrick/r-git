package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/testutil"
)

func TestArgvAndEnvironment(t *testing.T) {
	r := NewForTesting("git", Version{Major: 2, Minor: 40}, []string{"PATH=/bin", "GIT_DIR=/bad", "GIT_INDEX_FILE=/bad-index", "KEEP=yes"})
	c := Cmd{Subcommand: "log", Args: []string{"--oneline"}, Config: []string{"core.commentChar=;"}, NoPager: true, NoOptionalLocks: true, Env: map[string]string{"EXTRA": "value"}}
	want := []string{"-c", "core.quotepath=false", "-c", "log.showSignature=false", "-c", "core.commentChar=;", "--no-pager", "--no-optional-locks", "log", "--oneline"}
	got := r.argv(c)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
	env := strings.Join(r.environment(c.Env), "\n")
	for _, required := range []string{"LC_ALL=C", "LANGUAGE=", "GIT_TERMINAL_PROMPT=0", "EXTRA=value", "KEEP=yes"} {
		if !strings.Contains(env, required) {
			t.Errorf("environment lacks %q: %s", required, env)
		}
	}
	for _, forbidden := range []string{"GIT_DIR=", "GIT_INDEX_FILE="} {
		if strings.Contains(env, forbidden) {
			t.Errorf("environment contains %q: %s", forbidden, env)
		}
	}
}

func TestRunAndStream(t *testing.T) {
	repo := testutil.NewRepo(t)
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Run(context.Background(), Cmd{Dir: repo.Dir, Subcommand: "status", Args: []string{"--short"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	repo.WriteFile("tracked.txt", "line\n")
	repo.Commit("tracked")
	var lines []Line
	result, err = r.Stream(context.Background(), Cmd{Dir: repo.Dir, Subcommand: "show", Args: []string{"-s", "--format=%s", "HEAD"}}, func(line Line) { lines = append(lines, line) })
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(result.Stdout)); got != "tracked" {
		t.Fatalf("stdout = %q", got)
	}
	if len(lines) == 0 || lines[0].Stream != Stdout || lines[0].Text != "tracked" {
		t.Fatalf("stream lines = %#v", lines)
	}
	_, err = r.Run(context.Background(), Cmd{Dir: repo.Dir, Subcommand: "not-a-real-command"})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code == 0 || exitErr.Command == "" || exitErr.Stderr == "" {
		t.Fatalf("expected populated ExitError, got %#v (%v)", exitErr, err)
	}
}

func TestRunCancellation(t *testing.T) {
	repo := testutil.NewRepo(t)
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	_, err = r.Run(ctx, Cmd{Dir: repo.Dir, Subcommand: "block", Config: []string{"alias.block=!sleep 5"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
}

func TestTextSplitterPortedCases(t *testing.T) {
	cases := []struct {
		chunks []string
		want   []string
	}{
		{[]string{"line1\nline2\n"}, []string{"line1", "line2"}},
		{[]string{"line1\rline2\r"}, []string{"line1!", "line2!"}},
		{[]string{"line1\r\nline2\r\n"}, []string{"line1", "line2"}},
		{[]string{"lineLF\nlineCR\rlineCRLF\r\nlineLFCR\n\r"}, []string{"lineLF", "lineCR!", "lineCRLF", "lineLFCR", "!"}},
		{[]string{"lineLF\npartial"}, []string{"lineLF", "partial"}},
		{[]string{"start", "end\n"}, []string{"startend"}},
		{[]string{"lineCR\r", "\nlineLF"}, []string{"lineCR", "lineLF"}},
		{[]string{"lineCR\r", "\rlineLF"}, []string{"lineCR!", "!", "lineLF"}},
		{[]string{"lineLF\n", "lineLF"}, []string{"lineLF", "lineLF"}},
		{[]string{"lineCR\r", "lineLF"}, []string{"lineCR!", "lineLF"}},
		{[]string{"lineLF", "\nlineLF"}, []string{"lineLF", "lineLF"}},
		{[]string{"lineCR", "\rlineLF"}, []string{"lineCR!", "lineLF"}},
		{[]string{"lineLF", "\r\nlineLF"}, []string{"lineLF", "lineLF"}},
		{[]string{"\r\r\r"}, []string{"!", "!", "!"}},
		{[]string{"\n\n\n"}, []string{"", "", ""}},
		{[]string{"\r\n\r\n\r\n"}, []string{"", "", ""}},
		{[]string{"\r\r\n\n\n"}, []string{"!", "", "", ""}},
	}
	for _, tc := range cases {
		var got []string
		s := NewTextSplitter(func(line string, cr bool) {
			if cr {
				line += "!"
			}
			got = append(got, line)
		})
		for _, chunk := range tc.chunks {
			s.Process([]byte(chunk))
		}
		s.Flush()
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("chunks %#v => %#v; want %#v", tc.chunks, got, tc.want)
		}
	}
}

func TestVersionParsing(t *testing.T) {
	for input, want := range map[string]Version{"git version 2.43.0.windows.1\n": {Major: 2, Minor: 43, Patch: 0, Build: 1}, "git version 2.17.3": {Major: 2, Minor: 17, Patch: 3}, "git version 3.1": {Major: 3, Minor: 1}} {
		got, err := ParseVersion(input)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("ParseVersion(%q) = %#v, want %#v", input, got, want)
		}
	}
	if _, err := ParseVersion("unknown"); err == nil {
		t.Fatal("expected invalid version error")
	}
}

func TestDetectorsUseJavaMessages(t *testing.T) {
	local := []string{"error: Your local changes to the following files would be overwritten by merge:", "\tfile one.txt", "\tdir/file2", "Please commit your changes or stash them before you merge.", "Merge with strategy ort failed."}
	var localErr *ErrLocalChangesWouldBeOverwritten
	if err := DetectLocalChangesWouldBeOverwritten(local, Merge); !errors.As(err, &localErr) {
		t.Fatalf("local detection = %v", err)
	}
	if strings.Join(localErr.Files, "|") != "dir/file2|file one.txt" {
		t.Fatalf("local files = %#v", localErr.Files)
	}
	oldLocal := []string{"Your local changes to 'file.txt' would be overwritten by merge"}
	if err := DetectLocalChangesWouldBeOverwritten(oldLocal, Merge); !errors.As(err, &localErr) || localErr.Files[0] != "file.txt" {
		t.Fatalf("old local detection = %v", err)
	}
	oldCheckout := []string{"You have local changes to 'a.txt'; cannot switch branches"}
	if err := DetectLocalChangesWouldBeOverwritten(oldCheckout, Checkout); !errors.As(err, &localErr) || localErr.Files[0] != "a.txt" {
		t.Fatalf("old checkout detection = %v", err)
	}
	oldReset := []string{"error: Entry 'reset.txt' not uptodate. Cannot merge."}
	if err := DetectLocalChangesWouldBeOverwritten(oldReset, Reset); !errors.As(err, &localErr) || localErr.Files[0] != "reset.txt" {
		t.Fatalf("old reset detection = %v", err)
	}
	singleLine := []string{"error: Your local changes to the following files would be overwritten by checkout:", "  file1 dir/file2 dir/subdir/file3", "Please commit your changes or stash them before you switch branches."}
	if err := DetectLocalChangesWouldBeOverwritten(singleLine, Checkout); !errors.As(err, &localErr) || strings.Join(localErr.Files, "|") != "dir/file2|dir/subdir/file3|file1" {
		t.Fatalf("single-line files = %v", err)
	}
	untracked := []string{"error: The following untracked working tree files would be overwritten by merge:", "\tnew.txt", "Please move or remove them before you merge.", "Merge with strategy ort failed."}
	var untrackedErr *ErrUntrackedFilesWouldBeOverwritten
	if err := DetectUntrackedFilesWouldBeOverwritten(untracked); !errors.As(err, &untrackedErr) || untrackedErr.Files[0] != "new.txt" {
		t.Fatalf("untracked detection = %v", err)
	}
	if err := DetectUntrackedFilesWouldBeOverwritten([]string{"Untracked working tree file 'old.txt' would be overwritten by merge."}); !errors.As(err, &untrackedErr) || untrackedErr.Files[0] != "old.txt" {
		t.Fatalf("old untracked detection = %v", err)
	}
	var branchErr *ErrBranchCheckedOutInWorktree
	if err := DetectBranchCheckedOutInWorktree([]string{"fatal: 'feature' is already used by worktree at '/other/worktree/path'"}); !errors.As(err, &branchErr) || branchErr.Path != "/other/worktree/path" {
		t.Fatalf("worktree detection = %v", err)
	}
	if !DetectSimpleEvent([]string{"Already up to date"}, AlreadyUpToDate) || DetectSimpleEvent([]string{"nothing"}, AlreadyUpToDate) {
		t.Fatal("simple event matching differs from Java")
	}
}

func TestProgressParsing(t *testing.T) {
	a := NewProgressAnalyzer()
	steps := []struct {
		line, stage string
		want        float64
	}{
		{"Counting objects: 1000, done.", "counting", .01},
		{"Compressing objects: 50% (2/4)", "compressing", .1},
		{"Receiving objects: 45% (9/20)", "receiving", .51},
		{"Resolving deltas: 20% (2/10)", "resolving", .96},
		{"Writing objects: 50% (2/4)", "receiving", .56},
	}
	for _, step := range steps {
		got, ok := a.Analyze(step.line)
		if !ok || got.Stage != step.stage || (got.Fraction-step.want > 1e-9 || step.want-got.Fraction > 1e-9) {
			t.Errorf("Analyze(%q) = %#v,%v; want stage %s fraction %v", step.line, got, ok, step.stage, step.want)
		}
	}
	if _, ok := ParseProgress("not progress"); ok {
		t.Fatal("unmatched line parsed as progress")
	}
}

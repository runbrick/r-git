package commit

import (
	"errors"
	"fmt"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
)

func TestUserMessage(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{ops.ErrEmptyCommit, "Nothing to commit. Stage a change or include a file."},
		{fmt.Errorf("commit: %w", ops.ErrEmptyCommit), "Nothing to commit. Stage a change or include a file."},
		{ops.ErrGPGSign, "GPG could not sign the commit."},
		{ops.ErrUnmergedFiles, "Unmerged files must be resolved before committing."},
		{&command.ErrLocalChangesWouldBeOverwritten{Files: []string{"a.txt"}}, "Local changes would be overwritten."},
		{&command.ExitError{Code: 1, Command: "git commit", Stderr: "a lock exists\nmore"}, "a lock exists"},
		{errors.New("plain"), "plain"},
	}
	for _, tc := range cases {
		if got := UserMessage(tc.err); got != tc.want {
			t.Errorf("UserMessage(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

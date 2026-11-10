package commit

import (
	"errors"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
)

// UserMessage turns a git operation error into one sentence the commit
// panel can show. Typed errors from ops and command keep their meaning;
// a raw exit error is reduced to git's own stderr when it has one.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ops.ErrEmptyCommit):
		return "Nothing to commit. Stage a change or include a file."
	case errors.Is(err, ops.ErrGPGSign):
		return "GPG could not sign the commit."
	case errors.Is(err, ops.ErrUnmergedFiles):
		return "Unmerged files must be resolved before committing."
	case errors.Is(err, ops.ErrConflict):
		return "The operation stopped on a merge conflict."
	case errors.Is(err, ops.ErrAlreadyApplied):
		return "That commit is already applied."
	}
	var notMerged *ops.ErrNotFullyMerged
	if errors.As(err, &notMerged) {
		return notMerged.Error()
	}
	var local *command.ErrLocalChangesWouldBeOverwritten
	if errors.As(err, &local) {
		return "Local changes would be overwritten."
	}
	var untracked *command.ErrUntrackedFilesWouldBeOverwritten
	if errors.As(err, &untracked) {
		return "Untracked files would be overwritten."
	}
	var worktree *command.ErrBranchCheckedOutInWorktree
	if errors.As(err, &worktree) {
		return "The branch is already checked out in another worktree."
	}
	var exit *command.ExitError
	if errors.As(err, &exit) {
		text := strings.TrimSpace(exit.Stderr)
		if text != "" {
			if i := strings.IndexByte(text, '\n'); i >= 0 {
				text = text[:i]
			}
			return text
		}
	}
	return err.Error()
}

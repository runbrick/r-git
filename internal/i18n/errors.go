package i18n

import (
	"errors"
	"fmt"
	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/git/repo"
	"strings"
)

var ErrInvalidDiffPath = errors.New("invalid diff path")
var ErrBinaryDiff = errors.New("binary file: text diff is unavailable")
var ErrHistoryChanged = errors.New("rebase history changed; load the commits again")
var ErrNoCommitsRebase = errors.New("no commits to rebase")

func (text *Messages) Error(err error) string {
	if err == nil {
		return ""
	}
	var restore *ops.ErrCheckoutRestore
	if errors.As(err, &restore) {
		return fmt.Sprintf(text.CheckoutRestoreError, restore.Stash.Short(), restore.Cause)
	}
	switch {
	case errors.Is(err, ErrInvalidDiffPath):
		return text.InvalidDiffPath
	case errors.Is(err, ErrBinaryDiff):
		return text.BinaryDiff
	case errors.Is(err, ErrHistoryChanged):
		return text.HistoryChanged
	case errors.Is(err, ErrNoCommitsRebase):
		return text.NoCommitsRebase
	case errors.Is(err, repo.ErrNotRepository):
		return text.NotRepository
	case errors.Is(err, repo.ErrBareRepository):
		return text.BareRepository
	case errors.Is(err, config.ErrInvalidSettings):
		return text.InvalidSettings
	case errors.Is(err, config.ErrUnknownVersion):
		return text.UnknownSettingsVersion
	case errors.Is(err, ops.ErrInvalidTodo):
		return text.InvalidTodo
	case errors.Is(err, ops.ErrEmptyCommit):
		return text.NothingToCommitStageAChangeOrIncludeAFile
	case errors.Is(err, ops.ErrGPGSign):
		return text.GPGCouldNotSignTheCommit
	case errors.Is(err, ops.ErrUnmergedFiles):
		return text.UnmergedFilesMustBeResolvedBeforeCommitting
	case errors.Is(err, ops.ErrConflict):
		return text.TheOperationStoppedOnAMergeConflict
	case errors.Is(err, ops.ErrAlreadyApplied):
		return text.ThatCommitIsAlreadyApplied
	}
	var notMerged *ops.ErrNotFullyMerged
	if errors.As(err, &notMerged) {
		return text.NotFullyMerged(notMerged.Branch, notMerged.Base)
	}
	var local *command.ErrLocalChangesWouldBeOverwritten
	if errors.As(err, &local) {
		return text.LocalChangesWouldBeOverwritten
	}
	var untracked *command.ErrUntrackedFilesWouldBeOverwritten
	if errors.As(err, &untracked) {
		return text.UntrackedFilesWouldBeOverwritten
	}
	var worktree *command.ErrBranchCheckedOutInWorktree
	if errors.As(err, &worktree) {
		return text.TheBranchIsAlreadyCheckedOutInAnotherWorktree
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

package ops

import (
	"context"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
)

// ResetMode is the git reset mode flag.
// The names match GitResetMode: SOFT, MIXED, HARD, KEEP.
type ResetMode int

const (
	// ResetSoft moves HEAD and leaves the index and work tree
	// (git reset --soft).
	ResetSoft ResetMode = iota
	// ResetMixed moves HEAD and the index, and leaves the work tree
	// (git reset --mixed). This is git's default reset.
	ResetMixed
	// ResetHard moves HEAD, the index, and the work tree
	// (git reset --hard).
	ResetHard
	// ResetKeep moves HEAD but refuses to discard local changes
	// (git reset --keep).
	ResetKeep
)

// Argument is the flag GitResetMode passes to git reset.
func (m ResetMode) Argument() string {
	switch m {
	case ResetSoft:
		return "--soft"
	case ResetMixed:
		return "--mixed"
	case ResetHard:
		return "--hard"
	case ResetKeep:
		return "--keep"
	default:
		return "--mixed"
	}
}

// Reset moves HEAD to target with mode and returns the new HEAD hash.
//
// When mode is ResetKeep or ResetMixed and local changes would be
// overwritten, the error is *command.ErrLocalChangesWouldBeOverwritten.
// Java's GitResetOperation offers a smart reset (stash, reset, unstash)
// in that case; ResetWithStash performs that sequence.
func (s *Service) Reset(ctx context.Context, mode ResetMode, target string) (model.Hash, error) {
	res, err := s.run(ctx, "reset", mode.Argument(), target)
	if err != nil {
		return model.Hash{}, classify(res, err, command.Reset, "")
	}
	return s.head(ctx)
}

// ResetWithStash stashes local changes, resets, then pops the stash.
// It is the smart-reset path of GitResetOperation: stash, git reset, unstash.
func (s *Service) ResetWithStash(ctx context.Context, mode ResetMode, target string) (model.Hash, error) {
	msg := preservingMessage("Reset", timeNow())
	if _, err := s.run(ctx, "stash", "push", "-m", msg); err != nil {
		return model.Hash{}, err
	}
	hash, resetErr := s.Reset(ctx, mode, target)
	popRes, popErr := s.run(ctx, "stash", "pop")
	if popErr != nil {
		popErr = classify(popRes, popErr, command.Merge, "")
		if resetErr != nil {
			return hash, resetErr
		}
		return hash, popErr
	}
	return hash, resetErr
}

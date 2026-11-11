package ops

import (
	"context"
	"fmt"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
)

// CherryPick applies hashes in the order given and returns the hashes of
// the commits it created.
//
// The order is the order git cherry-pick expects: oldest first, which is
// the reverse of git log. Each hash is applied with
// git cherry-pick <hash> (auto-commit, no -x), matching the default
// GitCherryPickProcess path when the published-commit suffix is off.
//
// A content conflict stops the sequence. The error wraps ErrConflict, and
// the returned hashes are the commits already created. Local changes that
// would be overwritten are returned as
// *command.ErrLocalChangesWouldBeOverwritten. An empty cherry-pick (the
// change is already present) wraps ErrAlreadyApplied and is skipped, which
// is the "one of which has already been applied" case: earlier and later
// commits in the list are still applied.
func (s *Service) CherryPick(ctx context.Context, hashes []string) ([]model.Hash, error) {
	return s.applyCommits(ctx, "cherry-pick", hashes, nil)
}

// Revert reverts hashes in the order given and returns the hashes of the
// revert commits. The order matches GitRevertProcess: the UI order, newest
// first, applied one commit at a time with git revert <hash> (auto-commit).
//
// A conflict stops the sequence and wraps ErrConflict. An empty revert
// (nothing to commit) is skipped and wraps nothing: the hash is omitted
// from the result and the sequence continues, matching
// "two commits reverted, one more was skipped because empty".
func (s *Service) Revert(ctx context.Context, hashes []string) ([]model.Hash, error) {
	return s.applyCommits(ctx, "revert", hashes, []string{"--no-edit"})
}

func (s *Service) applyCommits(ctx context.Context, sub string, hashes []string, extra []string) ([]model.Hash, error) {
	var created []model.Hash
	for _, hash := range hashes {
		before, err := s.head(ctx)
		if err != nil {
			return created, err
		}
		args := append([]string{}, extra...)
		args = append(args, hash)
		res, err := s.run(ctx, sub, args...)
		if err != nil {
			if sub == "cherry-pick" && isAlreadyApplied(res) {
				// git cherry-pick --skip when the patch is empty, so the
				// sequencer does not stay in progress. Java records the
				// commit as already picked and continues.
				if _, skipErr := s.run(ctx, sub, "--skip"); skipErr != nil {
					return created, fmt.Errorf("%w: %s", ErrAlreadyApplied, hash)
				}
				continue
			}
			if sub == "revert" && isEmptyCommit(res) {
				if _, skipErr := s.run(ctx, sub, "--skip"); skipErr != nil {
					// Older git aborts an empty revert without entering the
					// sequencer. The sequence can continue.
					continue
				}
				continue
			}
			return created, fmt.Errorf("%s %s: %w", sub, hash, classify(res, err, command.Merge))
		}
		after, err := s.head(ctx)
		if err != nil {
			return created, err
		}
		if !after.Equal(before) {
			created = append(created, after)
		}
	}
	return created, nil
}

func isAlreadyApplied(res command.Result) bool {
	text := string(res.Stdout) + "\n" + string(res.Stderr)
	return containsFold(text, "nothing to commit") ||
		containsFold(text, "nothing added to commit") ||
		containsFold(text, "The previous cherry-pick is now empty") ||
		containsFold(text, "previous cherry-pick is now empty")
}

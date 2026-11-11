package ops

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
)

// Branches runs checkout, branch creation, deletion, and rename.
type Branches struct{ *Service }

// Checkout switches HEAD to name.
// name is a branch, a tag, or any revision git checkout accepts.
//
// On success the current HEAD hash is returned.
// When local modifications would be overwritten, the error is
// *command.ErrLocalChangesWouldBeOverwritten and the caller can retry with
// CheckoutWithStash. Untracked files that would be overwritten, unmerged
// index entries, and a branch already checked out in another worktree are
// returned as the matching command errors.
func (b Branches) Checkout(ctx context.Context, name string) (model.Hash, error) {
	return b.checkout(ctx, []string{name})
}

// CheckoutNew creates name starting at startPoint and checks it out.
// An empty startPoint starts the branch at HEAD
// (git checkout -b name), matching GitCheckoutNewBranchOperation.
// A non-empty startPoint is git checkout -b name startPoint,
// matching GitImpl.checkout.
func (b Branches) CheckoutNew(ctx context.Context, name, startPoint string) (model.Hash, error) {
	args := []string{"-b", name}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	return b.checkout(ctx, args)
}

func (b Branches) checkout(ctx context.Context, args []string) (model.Hash, error) {
	res, err := b.run(ctx, "checkout", args...)
	if err != nil {
		return model.Hash{}, classify(res, err, command.Checkout)
	}
	return b.head(ctx)
}

// CheckoutWithStash stashes local changes, checks out name, then pops the
// stash. This is the stash/checkout/unstash sequence Java's
// GitPreservingProcess runs when the user accepts a smart checkout.
//
// The stash message is "<operation> at <timestamp>". An empty operation
// uses "Checkout". If restoring the stash fails, ErrCheckoutRestore identifies
// the saved changes; the checkout itself has completed. A conflicting restore
// leaves the unmerged index available to the conflict resolver, as in Java.
func (b Branches) CheckoutWithStash(ctx context.Context, name, operation string) (model.Hash, error) {
	if operation == "" {
		operation = "Checkout"
	}
	msg := preservingMessage(operation, time.Now())
	saved, err := b.run(ctx, "stash", "push", "-m", msg)
	if err != nil {
		return model.Hash{}, fmt.Errorf("stash before checkout: %w", err)
	}
	// Like GitStashChangesSaver, only restore a stash we actually created.
	// Otherwise a clean checkout could pop the user's older stash.
	if containsFold(string(saved.Stdout), "No local changes to save") {
		return b.Checkout(ctx, name)
	}
	stash, err := (Stash{b.Service}).tip(ctx)
	if err != nil {
		return model.Hash{}, err
	}
	hash, checkoutErr := b.Checkout(ctx, name)
	popRes, popErr := b.run(ctx, "stash", "pop", "stash@{0}")
	if popErr != nil {
		restoreErr := &ErrCheckoutRestore{Branch: name, Stash: stash, Cause: classify(popRes, popErr, command.Merge)}
		return hash, errors.Join(checkoutErr, restoreErr)
	}
	return hash, checkoutErr
}

// ErrCheckoutRestore means the saved changes could not be fully restored.
// Git keeps the stash on failure; Stash identifies its commit even if its
// ordinal later changes. Unwrap preserves errors.Is(err, ErrConflict).
type ErrCheckoutRestore struct {
	Branch string
	Stash  model.Hash
	Cause  error
}

func (e *ErrCheckoutRestore) Error() string {
	return fmt.Sprintf("Local changes could not be fully restored; stash kept (%s): %v", e.Stash.Short(), e.Cause)
}

func (e *ErrCheckoutRestore) Unwrap() error { return e.Cause }

// Delete removes the local branch name.
// force selects git branch -D; otherwise git branch -d.
//
// When the branch is not fully merged and force is false, the error is
// *ErrNotFullyMerged carrying the unmerged commits. Java shows those
// commits in a dialog and then force-deletes on confirmation; this method
// stops before the force delete so the UI can ask.
func (b Branches) Delete(ctx context.Context, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	res, err := b.run(ctx, "branch", flag, name)
	if err == nil {
		return nil
	}
	lines := append(append([]string{}, res.ErrorLines...), res.OutputLines...)
	if !command.DetectSimpleEvent(lines, command.BranchNotFullyMerged) {
		return classify(res, err, command.Checkout)
	}
	base := unmergedBase(lines)
	if base == "" {
		var headErr error
		base, headErr = b.currentBranch(ctx)
		if headErr != nil {
			return headErr
		}
		if base == "" {
			base = "HEAD"
		}
	}
	commits, logErr := b.logHashes(ctx, base+".."+name)
	if logErr != nil {
		commits = nil
	}
	return &ErrNotFullyMerged{Branch: name, Base: base, Commits: commits}
}

// unmergedBase reads the upstream name from git's
// "not yet merged to '<upstream>', even though it is merged to HEAD" warning.
// GitBranchNotMergedToUpstreamDetector uses the same pattern.
var unmergedBasePattern = regexp.MustCompile(`.*'(.*)', even though it is merged to.*`)

func unmergedBase(lines []string) string {
	for _, line := range lines {
		if m := unmergedBasePattern.FindStringSubmatch(line); m != nil {
			base := m[1]
			base = strings.TrimPrefix(base, "refs/heads/")
			base = strings.TrimPrefix(base, "refs/remotes/")
			return base
		}
	}
	return ""
}

// Rename renames old branch to new (git branch -m old new).
// When unsetUpstream is true, the upstream of the renamed branch is
// cleared afterwards (git branch --unset-upstream new). A missing upstream
// is ignored, matching GitRenameBranchOperation.
func (b Branches) Rename(ctx context.Context, oldName, newName string, unsetUpstream bool) error {
	if _, err := b.run(ctx, "branch", "-m", oldName, newName); err != nil {
		return err
	}
	if !unsetUpstream {
		return nil
	}
	res, err := b.run(ctx, "branch", "--unset-upstream", newName)
	if err == nil {
		return nil
	}
	if containsFold(string(res.Stderr), "has no upstream information") {
		return nil
	}
	return err
}

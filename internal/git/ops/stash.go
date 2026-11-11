package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
)

// StashSaveOptions controls git stash push.
//
// Message is the stash subject. An empty message lets git use its default.
// IncludeUntracked adds --include-untracked.
// KeepIndex adds --keep-index, which leaves the staged changes in the work
// tree after the stash is recorded.
type StashSaveOptions struct {
	Message          string
	IncludeUntracked bool
	KeepIndex        bool
}

// SaveStash records the local changes and returns the new stash hash
// (stash@{0}). The arguments follow createStashHandler: git stash push,
// then the optional flags, then --message.
func (st *Service) SaveStash(ctx context.Context, opts StashSaveOptions) (model.Hash, error) {
	args := []string{"push"}
	if opts.KeepIndex {
		args = append(args, "--keep-index")
	}
	if opts.IncludeUntracked {
		args = append(args, "--include-untracked")
	}
	msg := strings.TrimSpace(opts.Message)
	if msg != "" {
		args = append(args, "--message", msg)
	}
	if _, err := st.run(ctx, "stash", args...); err != nil {
		return model.Hash{}, err
	}
	return st.stashTip(ctx)
}

// PopStash applies ref and drops it on success (git stash pop).
// An empty ref pops stash@{0}. A conflict returns ErrConflict and leaves
// the stash in place, which is what git stash pop does.
func (st *Service) PopStash(ctx context.Context, ref string) error {
	return st.applyStash(ctx, "pop", ref)
}

// ApplyStash restores ref without dropping it (git stash apply).
// An empty ref applies stash@{0}.
func (st *Service) ApplyStash(ctx context.Context, ref string) error {
	return st.applyStash(ctx, "apply", ref)
}

func (st *Service) applyStash(ctx context.Context, sub, ref string) error {
	args := []string{sub}
	if ref != "" {
		args = append(args, ref)
	}
	res, err := st.run(ctx, "stash", args...)
	if err != nil {
		return classify(res, err, command.Merge, "")
	}
	return nil
}

// DropStash removes one stash entry (git stash drop <ref>).
// An empty ref drops stash@{0}.
func (st *Service) DropStash(ctx context.Context, ref string) error {
	args := []string{"drop"}
	if ref != "" {
		args = append(args, ref)
	}
	_, err := st.run(ctx, "stash", args...)
	return err
}

// ClearStashes removes every stash entry (git stash clear).
func (st *Service) ClearStashes(ctx context.Context) error {
	_, err := st.run(ctx, "stash", "clear")
	return err
}

// ShowStash returns the patch text of one stash (git stash show -p <ref>).
// An empty ref shows stash@{0}.
func (st *Service) ShowStash(ctx context.Context, ref string) (string, error) {
	args := []string{"show", "-p"}
	if ref != "" {
		args = append(args, ref)
	}
	res, err := st.Git.Run(ctx, command.Cmd{
		Dir:        st.Root,
		Subcommand: "stash",
		Args:       args,
		ReadOnly:   true,
	})
	if err != nil {
		return "", err
	}
	return string(res.Stdout), nil
}

func (st *Service) stashTip(ctx context.Context) (model.Hash, error) {
	res, err := st.Git.Run(ctx, command.Cmd{
		Dir:        st.Root,
		Subcommand: "rev-parse",
		Args:       []string{"stash@{0}"},
		ReadOnly:   true,
	})
	if err != nil {
		return model.Hash{}, fmt.Errorf("resolve stash@{0}: %w", err)
	}
	return model.ParseHash(strings.TrimSpace(string(res.Stdout)))
}

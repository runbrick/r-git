// Package ops runs Git operations in a repository: branch, index, commit,
// stash, reset, cherry-pick, revert, tag, fetch, pull, push, remote management,
// merge, conflict resolution, and rebase (including interactive rebase).
//
// Each operation is a method on Service. Service holds a command.Executor
// and the repository root. Methods take a context, run git, and return a
// typed result or a typed error. They do not talk to a UI. When Java would
// open a dialog (local changes would be overwritten, a branch is not fully
// merged), the method returns the matching error so the caller can decide.
package ops

import (
	"context"
	"errors"

	"github.com/runbrick/r-git/internal/git/command"
)

// Service runs Git operations in one repository. All operations are methods
// directly on Service; area-specific names disambiguate branch, stash, and tag
// methods without separate entry types.
type Service struct {
	Git  command.Executor
	Root string

	// Editor is the sequence/message editor executable for interactive rebase.
	// An empty value uses the current executable.
	Editor string
}

// New returns a service bound to root. git must be non-nil.
func New(git command.Executor, root string) *Service {
	return &Service{Git: git, Root: root}
}

// run executes one command in the repository root.
func (s *Service) run(ctx context.Context, sub string, args ...string) (command.Result, error) {
	return s.runWithEnv(ctx, sub, nil, args...)
}

func (s *Service) runWithEnv(ctx context.Context, sub string, env map[string]string, args ...string) (command.Result, error) {
	if s.Git == nil {
		return command.Result{}, errors.New("nil git executor")
	}
	return s.Git.Run(ctx, command.Cmd{
		Dir:        s.Root,
		Subcommand: sub,
		Args:       args,
		Env:        env,
		// Runner infers writes for the other mutating commands. These three
		// retain the explicit write locking used by the local operations.
		Write: sub == "branch" || sub == "tag" || sub == "clean",
	})
}

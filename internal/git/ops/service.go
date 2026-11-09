// Package ops runs local Git operations: branch, index, commit, stash,
// reset, cherry-pick, revert, and tag.
//
// Each operation is a method on Service. Service holds a command.Executor
// and the repository root. Methods take a context, run git, and return a
// typed result or a typed error. They do not talk to a UI. When Java would
// open a dialog (local changes would be overwritten, a branch is not fully
// merged), the method returns the matching error so the caller can decide.
package ops

import (
	"context"

	"github.com/runbrick/r-git/internal/git/command"
)

// Service runs local Git operations in one repository.
type Service struct {
	Git  command.Executor
	Root string
}

// New returns a service bound to root. git must be non-nil.
func New(git command.Executor, root string) *Service {
	return &Service{Git: git, Root: root}
}

// run executes one write command in the repository root.
func (s *Service) run(ctx context.Context, sub string, args ...string) (command.Result, error) {
	return s.Git.Run(ctx, command.Cmd{
		Dir:        s.Root,
		Subcommand: sub,
		Args:       args,
		Write:      true,
	})
}

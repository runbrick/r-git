package ops

import (
	"context"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
)

// CommitOptions selects the commit git records.
//
// Message is the exact commit message bytes. It is written to a temporary
// file and passed with -F, the same way GitRepositoryCommitter does, so
// the bytes are not altered by argument encoding.
// Amend adds --amend (CommitToAmend.Last).
// SignOff adds --signoff.
// Author, when non-empty, is passed as --author=<Author>.
// Paths, when non-empty, limits the commit to those paths
// (git commit --only -- <paths>), which is the partial commit of the
// selected files. When Paths is empty, whatever is already staged is committed.
type CommitOptions struct {
	Message []byte
	Amend   bool
	SignOff bool
	Author  string
	Paths   []string
}

// Commit records a commit and returns the new HEAD hash.
//
// An empty commit (nothing staged, and not an amend) returns ErrEmptyCommit.
// A gpg signing failure returns ErrGPGSign.
func (s *Service) Commit(ctx context.Context, opts CommitOptions) (model.Hash, error) {
	msgArgs, msgPath, err := s.messageFileArgs(opts.Message)
	if err != nil {
		return model.Hash{}, err
	}
	defer removeFile(msgPath)

	args := append([]string{}, msgArgs...)
	if opts.Amend {
		args = append(args, "--amend")
	}
	if opts.SignOff {
		args = append(args, "--signoff")
	}
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--only", "--")
		args = append(args, opts.Paths...)
	}
	res, err := s.run(ctx, "commit", args...)
	if err != nil {
		return model.Hash{}, classify(res, err, command.Checkout)
	}
	return s.head(ctx)
}

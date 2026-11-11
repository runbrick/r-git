package ops

import (
	"context"

	"github.com/runbrick/r-git/internal/git/command"
)

type PullMethod int

const (
	PullMethodMerge PullMethod = iota
	PullMethodRebase
)

type PullParams struct {
	Remote     string
	Branch     string
	Method     PullMethod
	NoCommit   bool
	OnProgress func(command.Line)
}

func (p *Service) Pull(ctx context.Context, params PullParams) error {
	args := []string{"--progress"}
	if params.Method == PullMethodRebase {
		args = append(args, "--rebase")
	} else {
		args = append(args, "--no-rebase")
	}

	if params.NoCommit {
		args = append(args, "--no-commit")
	}

	if params.Remote != "" {
		args = append(args, params.Remote)
		if params.Branch != "" {
			args = append(args, params.Branch)
		}
	}

	cmd := command.Cmd{
		Subcommand: "pull",
		Args:       args,
		Dir:        p.Root,
	}

	if params.OnProgress != nil {
		_, err := p.Git.Stream(ctx, cmd, params.OnProgress)
		return err
	}

	_, err := p.Git.Run(ctx, cmd)
	return err
}

func (p *Service) Stash(ctx context.Context, message string) error {
	_, err := p.run(ctx, "stash", "push", "-m", message)
	return err
}

func (p *Service) Unstash(ctx context.Context) error {
	_, err := p.run(ctx, "stash", "pop")
	return err
}

func (p *Service) StashList(ctx context.Context) (bool, error) {
	res, err := p.run(ctx, "stash", "list")
	if err != nil {
		return false, err
	}
	return len(res.Stdout) > 0, nil
}

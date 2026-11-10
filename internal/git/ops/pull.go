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

type Puller struct {
	runner *command.Runner
}

func NewPuller(runner *command.Runner) *Puller {
	return &Puller{runner: runner}
}

type PullParams struct {
	RepoPath   string
	Remote     string
	Branch     string
	Method     PullMethod
	NoCommit   bool
	OnProgress func(command.Line)
}

func (p *Puller) Pull(ctx context.Context, params PullParams) error {
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
		Dir:        params.RepoPath,
	}

	if params.OnProgress != nil {
		_, err := p.runner.Stream(ctx, cmd, params.OnProgress)
		return err
	}

	_, err := p.runner.Run(ctx, cmd)
	return err
}

func (p *Puller) Stash(ctx context.Context, repoPath, message string) error {
	cmd := command.Cmd{
		Subcommand: "stash",
		Args:       []string{"push", "-m", message},
		Dir:        repoPath,
	}
	_, err := p.runner.Run(ctx, cmd)
	return err
}

func (p *Puller) Unstash(ctx context.Context, repoPath string) error {
	cmd := command.Cmd{
		Subcommand: "stash",
		Args:       []string{"pop"},
		Dir:        repoPath,
	}
	_, err := p.runner.Run(ctx, cmd)
	return err
}

func (p *Puller) StashList(ctx context.Context, repoPath string) (bool, error) {
	cmd := command.Cmd{
		Subcommand: "stash",
		Args:       []string{"list"},
		Dir:        repoPath,
	}
	res, err := p.runner.Run(ctx, cmd)
	if err != nil {
		return false, err
	}
	return len(res.Stdout) > 0, nil
}

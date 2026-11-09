package ops

import (
	"context"

	"github.com/runbrick/r-git/internal/git/command"
)

type RemoteManager struct {
	runner *command.Runner
}

func NewRemoteManager(runner *command.Runner) *RemoteManager {
	return &RemoteManager{runner: runner}
}

func (rm *RemoteManager) AddRemote(ctx context.Context, repoPath, name, url string) error {
	cmd := command.Cmd{
		Subcommand: "remote",
		Args:       []string{"add", name, url},
		Dir:        repoPath,
	}
	_, err := rm.runner.Run(ctx, cmd)
	return err
}

func (rm *RemoteManager) RemoveRemote(ctx context.Context, repoPath, name string) error {
	cmd := command.Cmd{
		Subcommand: "remote",
		Args:       []string{"remove", name},
		Dir:        repoPath,
	}
	_, err := rm.runner.Run(ctx, cmd)
	return err
}

func (rm *RemoteManager) RenameRemote(ctx context.Context, repoPath, oldName, newName string) error {
	cmd := command.Cmd{
		Subcommand: "remote",
		Args:       []string{"rename", oldName, newName},
		Dir:        repoPath,
	}
	_, err := rm.runner.Run(ctx, cmd)
	return err
}

func (rm *RemoteManager) SetRemoteURL(ctx context.Context, repoPath, name, url string) error {
	cmd := command.Cmd{
		Subcommand: "remote",
		Args:       []string{"set-url", name, url},
		Dir:        repoPath,
	}
	_, err := rm.runner.Run(ctx, cmd)
	return err
}

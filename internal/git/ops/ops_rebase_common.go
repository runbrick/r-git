package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
)

var ErrInvalidTodo = errors.New("invalid rebase todo")

type Operations struct {
	Dir    string
	Git    command.Executor
	Editor string
}

func (o Operations) run(ctx context.Context, subcommand string, args ...string) (command.Result, error) {
	if o.Git == nil {
		return command.Result{}, errors.New("nil git executor")
	}
	return o.Git.Run(ctx, command.Cmd{Dir: o.Dir, Subcommand: subcommand, Args: args})
}

func (o Operations) runWithEnv(ctx context.Context, subcommand string, env map[string]string, args ...string) (command.Result, error) {
	if o.Git == nil {
		return command.Result{}, errors.New("nil git executor")
	}
	return o.Git.Run(ctx, command.Cmd{Dir: o.Dir, Subcommand: subcommand, Args: args, Env: env})
}

type Outcome uint8

const (
	OutcomeDone Outcome = iota
	OutcomeConflicts
	OutcomeStopped
	OutcomeNothingToDo
	OutcomeError
)

type MergeResult struct {
	Outcome   Outcome
	Conflicts []Conflict
	Err       error
}
type RebaseResult struct {
	Outcome       Outcome
	Conflicts     []Conflict
	Err           error
	StoppedAction string
}

func (r MergeResult) Error() error  { return r.Err }
func (r RebaseResult) Error() error { return r.Err }

func isConflict(err error, output []byte) bool {
	var exit *command.ExitError
	return errors.As(err, &exit) && (strings.Contains(string(output), "CONFLICT") || strings.Contains(exit.Stderr, "CONFLICT"))
}

func opError(kind string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", kind, err)
}

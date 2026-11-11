package ops_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
)

// Result-returning operations retain their existing outcome rules rather than
// acquiring checkout/commit's broader typed-error detection during this refactor.
func TestServiceOutcomeErrorPolicy(t *testing.T) {
	for _, tt := range []struct {
		name    string
		output  string
		err     error
		outcome ops.Outcome
	}{
		{"done", "", nil, ops.OutcomeDone},
		{"stdout conflict", "CONFLICT (content)", &command.ExitError{Code: 1}, ops.OutcomeConflicts},
		{"exit stderr conflict", "", &command.ExitError{Code: 1, Stderr: "CONFLICT"}, ops.OutcomeConflicts},
		{"transport error", "CONFLICT", errors.New("transport failed"), ops.OutcomeError},
		{"lowercase conflict", "conflict", &command.ExitError{Code: 1}, ops.OutcomeError},
		{"already current", "", &command.ExitError{Code: 1, Stderr: "up to date"}, ops.OutcomeNothingToDo},
		{"cancellation", "", context.Canceled, ops.OutcomeError},
		{"executor sentinel", "", ops.ErrConflict, ops.OutcomeError},
	} {
		for _, rebase := range []bool{false, true} {
			operation := "merge"
			if rebase {
				operation = "rebase"
			}
			t.Run(operation+"/"+tt.name, func(t *testing.T) {
				s := ops.New(serviceExecutor{run: func(_ context.Context, cmd command.Cmd) (command.Result, error) {
					if cmd.Subcommand != operation {
						// Conflicts reads the unmerged index after a conflict.
						return command.Result{}, nil
					}
					return command.Result{Stdout: []byte(tt.output)}, tt.err
				}}, "bound-repository")
				var outcome ops.Outcome
				var err error
				if rebase {
					result := s.Rebase(context.Background(), "side", ops.RebaseOptions{})
					outcome, err = result.Outcome, result.Err
				} else {
					result := s.Merge(context.Background(), "side", ops.MergeOptions{})
					outcome, err = result.Outcome, result.Err
				}
				if outcome != tt.outcome {
					t.Fatalf("outcome = %v, want %v; error = %v", outcome, tt.outcome, err)
				}
				if outcome == ops.OutcomeError {
					if !errors.Is(err, tt.err) || !strings.HasPrefix(err.Error(), operation+": ") {
						t.Fatalf("lost error cause or prefix: %v", err)
					}
				} else if err != nil {
					t.Fatalf("unexpected error for outcome %v: %v", outcome, err)
				}
			})
		}
	}
}

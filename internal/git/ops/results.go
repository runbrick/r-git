package ops

import "errors"

var ErrInvalidTodo = errors.New("invalid rebase todo")

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

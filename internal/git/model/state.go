package model

import (
	"fmt"
	"strconv"
)

// RepoState is the repository state (Repository.State).
//
// The names match the Java enum. GRAFTING is the cherry-pick state: Git
// has no separate "cherry-picking" value in this enum, and the Java
// comment documents GRAFTING as "during cherry-pick/grafting".
type RepoState int

const (
	// StateNormal means HEAD is on a branch and no merge, rebase,
	// cherry-pick, or revert is in progress.
	StateNormal RepoState = iota
	// StateMerging means a merge is in progress.
	StateMerging
	// StateRebasing means a rebase is in progress.
	StateRebasing
	// StateGrafting means a cherry-pick (graft) is in progress.
	StateGrafting
	// StateReverting means a revert is in progress.
	StateReverting
	// StateDetached means HEAD is detached and no rebase is in progress.
	StateDetached
)

// String returns the Java enum name: NORMAL, MERGING, REBASING, GRAFTING,
// REVERTING, DETACHED.
func (s RepoState) String() string {
	switch s {
	case StateNormal:
		return "NORMAL"
	case StateMerging:
		return "MERGING"
	case StateRebasing:
		return "REBASING"
	case StateGrafting:
		return "GRAFTING"
	case StateReverting:
		return "REVERTING"
	case StateDetached:
		return "DETACHED"
	default:
		return "RepoState(" + strconv.Itoa(int(s)) + ")"
	}
}

// ParseRepoState parses a Java enum name. The match is case-sensitive.
func ParseRepoState(s string) (RepoState, error) {
	switch s {
	case "NORMAL":
		return StateNormal, nil
	case "MERGING":
		return StateMerging, nil
	case "REBASING":
		return StateRebasing, nil
	case "GRAFTING":
		return StateGrafting, nil
	case "REVERTING":
		return StateReverting, nil
	case "DETACHED":
		return StateDetached, nil
	default:
		return 0, fmt.Errorf("unknown repository state %q", s)
	}
}

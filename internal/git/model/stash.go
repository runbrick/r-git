package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidStashRef is returned when a string is not a stash reflog
// selector of the form stash@{n}.
var ErrInvalidStashRef = errors.New("invalid stash ref")

// Stash is one entry in the stash stack (StashInfo), without a repository
// root.
//
// Index is n in stash@{n}, with 0 being the most recent. Ref is the
// selector itself ("stash@{0}"). Hash is the stash commit. Branch is the
// branch the stash was created on, as Git records it in the subject
// ("WIP on feature", or empty when the selector line has no branch).
// Message is the stash subject. When is the author time.
type Stash struct {
	Index   int
	Ref     string
	Hash    Hash
	Branch  string
	Message string
	When    time.Time
}

// Equal compares index, ref, hash, branch, message, and time.
func (s Stash) Equal(other Stash) bool {
	return s.Index == other.Index && s.Ref == other.Ref && s.Hash.Equal(other.Hash) &&
		s.Branch == other.Branch && s.Message == other.Message && s.When.Equal(other.When)
}

// ParseStashRef parses "stash@{n}" into n. The braces and the "stash@"
// prefix are required. A leading "refs/stash" form is not accepted; Git's
// short reflog selector is the form StashInfo stores.
func ParseStashRef(ref string) (int, error) {
	const prefix = "stash@{"
	if !strings.HasPrefix(ref, prefix) || !strings.HasSuffix(ref, "}") {
		return 0, fmt.Errorf("%w: %q", ErrInvalidStashRef, ref)
	}
	num := ref[len(prefix) : len(ref)-1]
	if num == "" {
		return 0, fmt.Errorf("%w: %q", ErrInvalidStashRef, ref)
	}
	for _, c := range num {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%w: %q", ErrInvalidStashRef, ref)
		}
	}
	// Reject a leading zero on a multi-digit index so "stash@{01}" is not
	// silently treated as a different spelling of "stash@{1}". Git prints
	// the index without padding.
	if len(num) > 1 && num[0] == '0' {
		return 0, fmt.Errorf("%w: %q", ErrInvalidStashRef, ref)
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidStashRef, ref)
	}
	return n, nil
}

// FormatStashRef returns "stash@{n}". A negative n is rejected.
func FormatStashRef(n int) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("%w: index %d", ErrInvalidStashRef, n)
	}
	return fmt.Sprintf("stash@{%d}", n), nil
}

// Worktree is one working tree of a repository (GitWorkingTree), plus the
// two facts the task asks for that the Java type leaves implicit: whether
// the tree is bare, and whether HEAD is detached.
//
// Path is the working tree root. HEAD is the checked-out commit when it is
// known. Branch is the current local branch; it is the zero Ref when the
// tree is detached or bare. Locked and Prunable match git worktree list:
// a locked tree is kept, a prunable tree has stale administrative files.
// Bare is true for a repository with no work tree. Detached is true when
// HEAD points at a commit rather than a branch. Main is the primary work
// tree (the one that owns .git rather than .git/worktrees/<id>).
type Worktree struct {
	Path     string
	HEAD     Hash
	Branch   Ref
	Locked   bool
	Prunable bool
	Bare     bool
	Detached bool
	Main     bool
}

// OnBranch reports whether the work tree has a current local branch.
func (w Worktree) OnBranch() bool {
	return w.Branch.Kind() == KindLocalBranch && !w.Detached && !w.Bare
}

// Equal compares every field.
func (w Worktree) Equal(other Worktree) bool {
	return w.Path == other.Path && w.HEAD.Equal(other.HEAD) && w.Branch.Equal(other.Branch) &&
		w.Locked == other.Locked && w.Prunable == other.Prunable &&
		w.Bare == other.Bare && w.Detached == other.Detached && w.Main == other.Main
}

// Submodule is one submodule entry (GitSubmoduleInfo): the path relative
// to the superproject root, and the URL from .gitmodules.
type Submodule struct {
	Path string
	URL  string
}

// Equal compares path and URL.
func (s Submodule) Equal(other Submodule) bool {
	return s.Path == other.Path && s.URL == other.URL
}

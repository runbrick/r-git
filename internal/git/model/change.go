package model

import (
	"errors"
	"fmt"
	"strings"
)

// FileStatus is the kind of change Git reports for one path.
//
// The letters match git diff --name-status / git status porcelain v1 and
// GitChangeType, plus the two porcelain states that are not history
// changes: untracked ('?') and ignored ('!'). Unmerged ('U') covers the
// conflicted entries GitChangeType calls UNRESOLVED.
type FileStatus int

const (
	StatusUnknown FileStatus = iota
	StatusAdded
	StatusModified
	StatusDeleted
	StatusRenamed
	StatusCopied
	StatusTypeChanged
	StatusUnmerged
	StatusUntracked
	StatusIgnored
)

// String returns a short name ("added", "renamed", ...).
func (s FileStatus) String() string {
	switch s {
	case StatusAdded:
		return "added"
	case StatusModified:
		return "modified"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusCopied:
		return "copied"
	case StatusTypeChanged:
		return "type-changed"
	case StatusUnmerged:
		return "unmerged"
	case StatusUntracked:
		return "untracked"
	case StatusIgnored:
		return "ignored"
	default:
		return "unknown"
	}
}

// Letter returns the git status letter ("A", "M", "D", "R", "C", "T",
// "U", "?", "!"). Unknown returns "".
func (s FileStatus) Letter() string {
	switch s {
	case StatusAdded:
		return "A"
	case StatusModified:
		return "M"
	case StatusDeleted:
		return "D"
	case StatusRenamed:
		return "R"
	case StatusCopied:
		return "C"
	case StatusTypeChanged:
		return "T"
	case StatusUnmerged:
		return "U"
	case StatusUntracked:
		return "?"
	case StatusIgnored:
		return "!"
	default:
		return ""
	}
}

// ErrUnknownStatus is returned for a status letter this package does not know.
var ErrUnknownStatus = errors.New("unknown git file status")

// ParseFileStatus reads a single status letter. A rename or copy score
// suffix ("R100", "C75") is accepted and ignored; the score is not part of
// the data model. The match is case-sensitive and uses the first character,
// matching GitChangeType.fromString.
func ParseFileStatus(letter string) (FileStatus, error) {
	if letter == "" {
		return StatusUnknown, fmt.Errorf("%w: empty", ErrUnknownStatus)
	}
	switch letter[0] {
	case 'A':
		return StatusAdded, nil
	case 'M':
		return StatusModified, nil
	case 'D':
		return StatusDeleted, nil
	case 'R':
		return StatusRenamed, nil
	case 'C':
		return StatusCopied, nil
	case 'T':
		return StatusTypeChanged, nil
	case 'U':
		return StatusUnmerged, nil
	case '?':
		return StatusUntracked, nil
	case '!':
		return StatusIgnored, nil
	default:
		return StatusUnknown, fmt.Errorf("%w: %q", ErrUnknownStatus, letter)
	}
}

// FileChange is one path changed between two trees, or one unmerged,
// untracked, or ignored path.
//
// Path is the new path for an add, modify, rename, or copy, and the only
// path for a delete. OrigPath is the old path, set for renames and copies
// and for unmerged entries that Git reports with an origin. It is empty
// otherwise.
//
// Index and Worktree are the porcelain v2 XY letters when the change came
// from status. '.' means that side is unchanged. History diffs leave both
// empty. A partially staged file has a non-dot letter on each side.
//
// For a rename, Path is the destination and OrigPath is the source. That
// matches git diff --name-status output order (orig, then new) once the
// caller has assigned the two columns; it also matches the task's
// path/origPath fields. VcsFileStatusInfo stores them in the opposite
// argument order (first = before, second = after); see OrigPath.
type FileChange struct {
	Status   FileStatus
	Path     string
	OrigPath string
	Index    byte
	Worktree byte
}

// Equal compares status and both paths.
func (c FileChange) Equal(other FileChange) bool {
	if c.Status != other.Status || c.Path != other.Path || c.OrigPath != other.OrigPath {
		return false
	}
	if (c.Index != 0 || c.Worktree != 0) && (other.Index != 0 || other.Worktree != 0) {
		return c.Index == other.Index && c.Worktree == other.Worktree
	}
	return true
}

// String formats the change as "<letter> <path>" or "<letter> <orig> -> <path>".
func (c FileChange) String() string {
	if c.OrigPath != "" {
		return c.Status.Letter() + " " + c.OrigPath + " -> " + c.Path
	}
	return c.Status.Letter() + " " + c.Path
}

// ConflictSide is which side of a conflict a status belongs to, in Git's
// terms (--ours or --theirs). It ignores the IDE-level reversal of visible
// sides (GitConflict.ConflictSide).
type ConflictSide int

const (
	SideOurs ConflictSide = iota
	SideTheirs
)

// String returns "ours" or "theirs".
func (s ConflictSide) String() string {
	if s == SideTheirs {
		return "theirs"
	}
	return "ours"
}

// ConflictStageStatus is the per-side state of one unmerged path
// (GitConflict.Status): the stage was added, modified, or deleted.
type ConflictStageStatus int

const (
	ConflictModified ConflictStageStatus = iota
	ConflictDeleted
	ConflictAdded
)

// String returns "modified", "deleted", or "added".
func (s ConflictStageStatus) String() string {
	switch s {
	case ConflictDeleted:
		return "deleted"
	case ConflictAdded:
		return "added"
	default:
		return "modified"
	}
}

// Conflict describes one unmerged path (GitConflict), without a repository
// root. Ours and Theirs are the stage statuses. Reversed selects the
// IDE-level side swap (GitMergeUtil.isReverseRoot): when it is set,
// Status(SideOurs) returns Theirs.
type Conflict struct {
	Path     string
	Ours     ConflictStageStatus
	Theirs   ConflictStageStatus
	Reversed bool
}

// Status returns the stage status for side, swapping the sides when the
// conflict is reversed. This matches GitConflict.getStatus(side, isReversed).
func (c Conflict) Status(side ConflictSide) ConflictStageStatus {
	ours := c.Ours
	theirs := c.Theirs
	if c.Reversed {
		ours, theirs = theirs, ours
	}
	if side == SideOurs {
		return ours
	}
	return theirs
}

// Equal compares path, both stage statuses, and the reversed flag.
func (c Conflict) Equal(other Conflict) bool {
	return c.Path == other.Path && c.Ours == other.Ours && c.Theirs == other.Theirs && c.Reversed == other.Reversed
}

// NeedsOrigin reports whether a status carries a second path.
// Renames and copies do. Other statuses do not.
func NeedsOrigin(status FileStatus) bool {
	return status == StatusRenamed || status == StatusCopied
}

// TrimStatusToken drops a numeric score from "R100" or "C075", returning
// the letter token ParseFileStatus accepts. Other tokens are unchanged.
func TrimStatusToken(token string) string {
	if token == "" {
		return token
	}
	switch token[0] {
	case 'R', 'C':
		i := 1
		for i < len(token) && token[i] >= '0' && token[i] <= '9' {
			i++
		}
		if i == len(token) {
			return token[:1]
		}
	}
	return token
}

// ParseNameStatusLine parses one "XY\tpath" or "XY\torig\tpath" record
// from git diff --name-status -z is not handled; the separator is a tab,
// which is what the non-z form uses. A rename score is accepted.
func ParseNameStatusLine(line string) (FileChange, error) {
	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		return FileChange{}, fmt.Errorf("%w: %q", ErrUnknownStatus, line)
	}
	status, err := ParseFileStatus(TrimStatusToken(parts[0]))
	if err != nil {
		return FileChange{}, err
	}
	if NeedsOrigin(status) {
		if len(parts) < 3 {
			return FileChange{}, fmt.Errorf("%w: %s missing origin in %q", ErrUnknownStatus, status, line)
		}
		return FileChange{Status: status, OrigPath: parts[1], Path: parts[2]}, nil
	}
	return FileChange{Status: status, Path: parts[1]}, nil
}

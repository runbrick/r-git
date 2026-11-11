package repo

import (
	"github.com/runbrick/r-git/internal/git/model"
)

// ChangeGroup is one section of the local-changes tree. The order is the
// order the Commit tool window shows: staged, then unstaged, then
// unversioned, then conflicts.
type ChangeGroup int

const (
	GroupStaged ChangeGroup = iota
	GroupUnstaged
	GroupUnversioned
	GroupConflicts
)

// String returns the section title used in the changes tree.
func (g ChangeGroup) String() string {
	switch g {
	case GroupStaged:
		return "Staged"
	case GroupUnstaged:
		return "Unstaged"
	case GroupUnversioned:
		return "Unversioned"
	case GroupConflicts:
		return "Conflicts"
	default:
		return ""
	}
}

// ChangeEntry is one path in one changes-tree group.
//
// Index and Worktree are the porcelain v2 XY letters. A staged path has a
// non-dot index letter; an unstaged path has a non-dot worktree letter.
// A path that is both staged and unstaged appears once in each group, the
// same way the Commit tool window lists a partially staged file twice.
// Status is the letter that names the change in that group.
type ChangeEntry struct {
	Group    ChangeGroup
	Status   model.FileStatus
	Path     string
	OrigPath string
}

// GroupChanges splits status changes into the four Commit tool-window
// groups. Ignored paths are omitted. Index and Worktree come from
// porcelain v2; a path that is both staged and unstaged is listed once
// in each group.
func GroupChanges(changes []model.FileChange) []ChangeEntry {
	var entries []ChangeEntry
	for _, change := range changes {
		if change.Status == model.StatusIgnored || change.Worktree == '!' {
			continue
		}
		if change.Status == model.StatusUnmerged || change.Index == 'U' || change.Worktree == 'U' {
			entries = append(entries, ChangeEntry{Group: GroupConflicts, Status: model.StatusUnmerged, Path: change.Path, OrigPath: change.OrigPath})
			continue
		}
		if change.Status == model.StatusUntracked || change.Worktree == '?' {
			entries = append(entries, ChangeEntry{Group: GroupUnversioned, Status: model.StatusUntracked, Path: change.Path})
			continue
		}
		index, work := change.Index, change.Worktree
		if index == 0 && work == 0 {
			entries = append(entries, ChangeEntry{Group: GroupUnstaged, Status: change.Status, Path: change.Path, OrigPath: change.OrigPath})
			continue
		}
		addOrdinary(&entries, index, work, change.Path, change.OrigPath)
	}
	return entries
}

func addOrdinary(entries *[]ChangeEntry, index, work byte, path, orig string) {
	if index != '.' && index != ' ' {
		status, err := model.ParseFileStatus(string(index))
		if err != nil {
			status = model.StatusUnknown
		}
		*entries = append(*entries, ChangeEntry{Group: GroupStaged, Status: status, Path: path, OrigPath: orig})
	}
	if work != '.' && work != ' ' {
		status, err := model.ParseFileStatus(string(work))
		if err != nil {
			status = model.StatusUnknown
		}
		group := GroupUnstaged
		if status == model.StatusUntracked {
			group = GroupUnversioned
		}
		*entries = append(*entries, ChangeEntry{Group: group, Status: status, Path: path, OrigPath: orig})
	}
}

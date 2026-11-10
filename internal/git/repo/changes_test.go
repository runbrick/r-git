package repo

import (
	"testing"

	"github.com/runbrick/r-git/internal/git/model"
)

func TestGroupChanges(t *testing.T) {
	changes := []model.FileChange{
		{Status: model.StatusModified, Path: "staged.txt", Index: 'M', Worktree: '.'},
		{Status: model.StatusModified, Path: "both.txt", Index: 'M', Worktree: 'M'},
		{Status: model.StatusModified, Path: "work.txt", Index: '.', Worktree: 'M'},
		{Status: model.StatusUntracked, Path: "new.txt", Worktree: '?'},
		{Status: model.StatusUnmerged, Path: "conflict.txt", Index: 'U', Worktree: 'U'},
		{Status: model.StatusIgnored, Path: "skip.txt", Worktree: '!'},
	}
	got := GroupChanges(changes)
	want := []ChangeEntry{
		{Group: GroupStaged, Status: model.StatusModified, Path: "staged.txt"},
		{Group: GroupStaged, Status: model.StatusModified, Path: "both.txt"},
		{Group: GroupUnstaged, Status: model.StatusModified, Path: "both.txt"},
		{Group: GroupUnstaged, Status: model.StatusModified, Path: "work.txt"},
		{Group: GroupUnversioned, Status: model.StatusUntracked, Path: "new.txt"},
		{Group: GroupConflicts, Status: model.StatusUnmerged, Path: "conflict.txt"},
	}
	if len(got) != len(want) {
		t.Fatalf("groups = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

package command

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type ErrLocalChangesWouldBeOverwritten struct{ Files []string }

func (e *ErrLocalChangesWouldBeOverwritten) Error() string {
	return fmt.Sprintf("local changes would be overwritten: %s", strings.Join(e.Files, ", "))
}

type ErrUntrackedFilesWouldBeOverwritten struct{ Files []string }

func (e *ErrUntrackedFilesWouldBeOverwritten) Error() string {
	return fmt.Sprintf("untracked files would be overwritten: %s", strings.Join(e.Files, ", "))
}

type ErrBranchCheckedOutInWorktree struct{ Branch, Path string }

func (e *ErrBranchCheckedOutInWorktree) Error() string {
	return fmt.Sprintf("branch %q is already checked out in worktree %q", e.Branch, e.Path)
}

type OverwriteOperation uint8

const (
	Checkout OverwriteOperation = iota
	Merge
	Reset
)

var (
	newLocalStart     = regexp.MustCompile(`.*Your local changes to the following files would be overwritten by.*`)
	newLocalEnd       = []*regexp.Regexp{regexp.MustCompile(`.*commit your changes or stash them before.*`), regexp.MustCompile(`.*Merge with strategy .* failed.*`), regexp.MustCompile(`.*No merge strategy handled the merge.*`)}
	oldCheckout       = regexp.MustCompile(`.*You have local changes to '(.*)'; cannot switch branches.*`)
	oldMerge          = regexp.MustCompile(`.*Your local changes to '(.*)' would be overwritten by merge.*`)
	oldReset          = []*regexp.Regexp{regexp.MustCompile(`.*Entry '(.*)' not uptodate. Cannot merge.*`), regexp.MustCompile(`.*Entry '(.*)' would be overwritten by merge.*`)}
	newUntrackedStart = regexp.MustCompile(`.*The following untracked working tree files would be overwritten by.*`)
	newUntrackedEnd   = []*regexp.Regexp{regexp.MustCompile(`.*Please move or remove them before.*`), regexp.MustCompile(`.*Merge with strategy .* failed.*`), regexp.MustCompile(`.*No merge strategy handled the merge.*`)}
	oldUntracked      = regexp.MustCompile(`.*Untracked working tree file '(.*)' would be overwritten by.*`)
	worktreeBranch    = regexp.MustCompile(`.*fatal: '(.*)' is already used by worktree at '(.*)'.*`)
)

func DetectLocalChangesWouldBeOverwritten(lines []string, operation OverwriteOperation) error {
	files, active := map[string]struct{}{}, false
	var rawLines []string
	for _, line := range lines {
		if newLocalStart.MatchString(line) {
			active = true
			continue
		}
		if matchesAny(newLocalEnd, line) {
			active = false
			continue
		}
		if active {
			if operation == Merge && strings.HasPrefix(line, "<stdin>:") && len(files) == 1 {
				active = false
				continue
			}
			rawLines = append(rawLines, line)
			files[strings.TrimSpace(line)] = struct{}{}
		}
		var patterns []*regexp.Regexp
		switch operation {
		case Checkout:
			patterns = []*regexp.Regexp{oldCheckout}
		case Merge:
			patterns = []*regexp.Regexp{oldMerge}
		case Reset:
			patterns = oldReset
		}
		for _, pattern := range patterns {
			if m := pattern.FindStringSubmatch(line); m != nil {
				files[m[1]] = struct{}{}
				break
			}
		}
	}
	if len(files) == 0 {
		return nil
	}
	list := sortedSet(files)
	if len(list) == 1 && len(rawLines) == 1 && strings.HasPrefix(rawLines[0], "  ") && len(rawLines[0]) > 2 && rawLines[0][2] != ' ' {
		list = strings.Fields(strings.TrimSpace(rawLines[0]))
		sort.Strings(list)
	}
	return &ErrLocalChangesWouldBeOverwritten{Files: list}
}

func DetectUntrackedFilesWouldBeOverwritten(lines []string) error {
	files, active := map[string]struct{}{}, false
	for _, line := range lines {
		if newUntrackedStart.MatchString(line) {
			active = true
			continue
		}
		if matchesAny(newUntrackedEnd, line) {
			active = false
			continue
		}
		if active {
			files[strings.TrimSpace(line)] = struct{}{}
		}
		if m := oldUntracked.FindStringSubmatch(line); m != nil {
			files[m[1]] = struct{}{}
		}
	}
	if len(files) == 0 {
		return nil
	}
	return &ErrUntrackedFilesWouldBeOverwritten{Files: sortedSet(files)}
}

func DetectBranchCheckedOutInWorktree(lines []string) error {
	for _, line := range lines {
		if m := worktreeBranch.FindStringSubmatch(line); m != nil {
			return &ErrBranchCheckedOutInWorktree{Branch: m[1], Path: m[2]}
		}
	}
	return nil
}

func matchesAny(patterns []*regexp.Regexp, line string) bool {
	for _, p := range patterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}
func sortedSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sortStrings(out)
	return out
}
func sortStrings(values []string) { sort.Strings(values) }

type SimpleEvent string

const (
	CherryPickConflict                  SimpleEvent = "after resolving the conflicts"
	LocalChangesOverwrittenByCherryPick SimpleEvent = "would be overwritten by "
	UnmergedPreventingCheckout          SimpleEvent = "you need to resolve your current index first"
	UnmergedPreventingMerge             SimpleEvent = "is not possible because you have unmerged files"
	BranchNotFullyMerged                SimpleEvent = "is not fully merged"
	MergeConflict                       SimpleEvent = "Automatic merge failed; fix conflicts and then commit the result"
	MergeConflictOnUnstash              SimpleEvent = "conflict"
	IndexConflictOnUnstash              SimpleEvent = "conflicts in index"
	AlreadyUpToDate                     SimpleEvent = "Already up-to-date"
	InvalidReference                    SimpleEvent = "invalid reference:"
)

func DetectSimpleEvent(lines []string, event SimpleEvent) bool {
	needles := []string{string(event)}
	switch event {
	case CherryPickConflict:
		needles = []string{"after resolving the conflicts", "CONFLICT (content): Merge conflict"}
	case AlreadyUpToDate:
		needles = append(needles, "Already up to date")
	}
	for _, line := range lines {
		for _, needle := range needles {
			if strings.Contains(strings.ToLower(line), strings.ToLower(needle)) {
				return true
			}
		}
	}
	return false
}

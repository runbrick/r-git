package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/runbrick/r-git/internal/git/model"
)

// StatusResult contains porcelain changes and upstream divergence counts.
type StatusResult struct {
	Changes       []model.FileChange
	Ahead, Behind int
}

// Status uses porcelain v2 because its NUL records preserve arbitrary path
// bytes (including newlines) and provide separate rename origin records.
func (r *Repo) Status(ctx context.Context) (StatusResult, error) {
	out, err := r.run(ctx, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return StatusResult{}, err
	}
	return ParseStatusV2(out.Stdout)
}

// ParseStatusV2 parses `git status --porcelain=v2 -z --branch` output.
func ParseStatusV2(data []byte) (StatusResult, error) {
	var result StatusResult
	items := strings.Split(string(data), "\x00")
	for i := 0; i < len(items); i++ {
		item := items[i]
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "# ") {
			if strings.HasPrefix(item, "# branch.ab ") {
				for _, f := range strings.Fields(strings.TrimPrefix(item, "# branch.ab ")) {
					if strings.HasPrefix(f, "+") {
						fmt.Sscanf(f, "+%d", &result.Ahead)
					}
					if strings.HasPrefix(f, "-") {
						fmt.Sscanf(f, "-%d", &result.Behind)
					}
				}
			}
			continue
		}
		fieldCount := 9
		if item[0] == 'u' {
			fieldCount = 11
		} else if item[0] == '2' {
			fieldCount = 10
		}
		fields := strings.SplitN(item, " ", fieldCount)
		switch item[0] {
		case '?':
			result.Changes = append(result.Changes, model.FileChange{Status: model.StatusUntracked, Path: item[2:], Worktree: '?'})
		case '!':
			result.Changes = append(result.Changes, model.FileChange{Status: model.StatusIgnored, Path: item[2:], Worktree: '!'})
		case '1':
			if len(fields) < 9 {
				return result, fmt.Errorf("malformed porcelain v2 ordinary record %q", item)
			}
			status, err := statusFromXY(fields[1])
			if err != nil {
				return result, err
			}
			change := model.FileChange{Status: status, Path: fields[8]}
			if len(fields[1]) >= 2 {
				change.Index, change.Worktree = fields[1][0], fields[1][1]
			}
			result.Changes = append(result.Changes, change)
		case '2':
			if len(fields) < 10 {
				return result, fmt.Errorf("malformed porcelain v2 rename record %q", item)
			}
			status, err := model.ParseFileStatus(fields[1][:2])
			if err != nil {
				status, err = statusFromXY(fields[1])
			}
			if err != nil {
				return result, err
			}
			if i+1 >= len(items) {
				return result, fmt.Errorf("rename record missing origin path")
			}
			i++
			change := model.FileChange{Status: status, Path: fields[9], OrigPath: items[i]}
			if len(fields[1]) >= 2 {
				change.Index, change.Worktree = fields[1][0], fields[1][1]
			}
			result.Changes = append(result.Changes, change)
		case 'u':
			if len(fields) < 11 {
				return result, fmt.Errorf("malformed porcelain v2 unmerged record %q", item)
			}
			result.Changes = append(result.Changes, model.FileChange{Status: model.StatusUnmerged, Path: fields[10], Index: 'U', Worktree: 'U'})
		default:
			return result, fmt.Errorf("unknown porcelain v2 record %q", item)
		}
	}
	return result, nil
}

func statusFromXY(xy string) (model.FileStatus, error) {
	if strings.Contains(xy, "U") {
		return model.StatusUnmerged, nil
	}
	for _, c := range []byte(xy) {
		if c == '.' {
			continue
		}
		status, err := model.ParseFileStatus(string(c))
		if err == nil {
			return status, nil
		}
	}
	return model.StatusUnknown, fmt.Errorf("invalid porcelain status %q", xy)
}

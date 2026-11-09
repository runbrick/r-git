package repo

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/runbrick/r-git/internal/git/model"
)

// Stashes returns the stash reflog in newest-first order.
func (r *Repo) Stashes(ctx context.Context) ([]model.Stash, error) {
	out, err := r.run(ctx, "stash", "list", "--format=%gd%x00%H%x00%gs%x00%at%x00")
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.Trim(string(out.Stdout), "\x00\n"), "\x00")
	var stashes []model.Stash
	for i := 0; i+3 < len(fields); i += 4 {
		for j := i; j < i+4; j++ {
			fields[j] = strings.TrimSpace(fields[j])
		}
		index, err := model.ParseStashRef(fields[i])
		if err != nil {
			return nil, err
		}
		hash, err := model.ParseHash(fields[i+1])
		if err != nil {
			return nil, err
		}
		unix, err := strconv.ParseInt(fields[i+3], 10, 64)
		if err != nil {
			return nil, err
		}
		subject, branch, message := fields[i+2], "", fields[i+2]
		if strings.HasPrefix(subject, "On ") {
			if colon := strings.Index(subject, ": "); colon >= 0 {
				branch, message = subject[3:colon], subject[colon+2:]
			}
		} else if strings.HasPrefix(subject, "WIP on ") {
			if colon := strings.Index(subject, ": "); colon >= 0 {
				branch, message = subject[7:colon], subject[colon+2:]
			}
		}
		stashes = append(stashes, model.Stash{Index: index, Ref: fields[i], Hash: hash, Branch: branch, Message: message, When: time.Unix(unix, 0)})
	}
	return stashes, nil
}

// Worktrees lists all linked and primary working trees.
func (r *Repo) Worktrees(ctx context.Context) ([]model.Worktree, error) {
	out, err := r.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	trees, err := ParseWorktrees(out.Stdout)
	if err != nil {
		return nil, err
	}
	for i := range trees {
		trees[i].Main = filepath.Clean(trees[i].Path) == filepath.Clean(r.Path)
	}
	return trees, nil
}

// ParseWorktrees parses `git worktree list --porcelain` output.
func ParseWorktrees(data []byte) ([]model.Worktree, error) {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, nil
	}
	blocks := strings.Split(text, "\n\n")
	var result []model.Worktree
	for _, block := range blocks {
		var w model.Worktree
		for _, line := range strings.Split(block, "\n") {
			key, value, _ := strings.Cut(line, " ")
			switch key {
			case "worktree":
				w.Path = value
			case "HEAD":
				h, e := model.ParseHash(value)
				if e != nil {
					return nil, fmt.Errorf("invalid worktree HEAD: %w", e)
				}
				w.HEAD = h
			case "branch":
				w.Branch, _ = model.LocalBranch(value)
			case "detached":
				w.Detached = true
			case "bare":
				w.Bare = true
			case "locked":
				w.Locked = true
			case "prunable":
				w.Prunable = true
			}
		}
		result = append(result, w)
	}
	return result, nil
}

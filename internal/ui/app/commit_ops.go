package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/git/repo"
	commitview "github.com/runbrick/r-git/internal/ui/commit"
)

func (m *Model) handleCommitRequest(req commitview.Request) {
	if m.repo == nil || m.commit.Busy {
		return
	}
	switch req.Kind {
	case commitview.RequestShowDiff:
		return
	case commitview.RequestCommit, commitview.RequestStage, commitview.RequestUnstage, commitview.RequestDiscard:
	default:
		return
	}
	m.commit.Busy = true
	m.commit.Error = ""
	ctx := context.Background()
	generation, repository := m.generation, m.repo
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		out := loadResult{kind: resultCommit, generation: generation}
		service := ops.New(repository.Executor, repository.Path)
		switch req.Kind {
		case commitview.RequestStage:
			out.err = service.Stage(ctx, req.Paths)
			out.toast = m.Text().Staged
		case commitview.RequestUnstage:
			out.err = service.Unstage(ctx, req.Paths)
			out.toast = m.Text().Unstaged
		case commitview.RequestDiscard:
			out.err = service.Discard(ctx, req.Paths)
			out.toast = m.Text().Discarded
		case commitview.RequestCommit:
			// Partial commit, matching git4idea: stage only checked paths
			// that are still untracked (git commit --only refuses those),
			// then commit --only the checked paths. Staged content the
			// user left unchecked, including a partial hunk, stays in the
			// index. When the checked paths are exactly the staged set,
			// a plain commit records the index as it is.
			status, statusErr := repository.Status(ctx)
			if statusErr != nil {
				out.err = statusErr
				break
			}
			if err := service.Stage(ctx, untrackedAmong(req.Paths, status.Changes)); err != nil {
				out.err = err
				break
			}
			opts := ops.CommitOptions{
				Message: []byte(req.Message),
				Amend:   req.Amend,
				SignOff: req.SignOff,
				Author:  req.Author,
			}
			if !sameSet(req.Paths, stagedPaths(status.Changes)) && !mergeInProgress(repository.Path) {
				opts.Paths = append([]string{}, req.Paths...)
			}
			hash, err := service.Commit(ctx, opts)
			out.err = err
			if err == nil {
				out.committed = true
				out.toast = hash.Short()
				out.pushAfter = req.Push
			}
		}
		m.fillChanges(ctx, repository, &out)
		m.publish(&out)
	}()
}

func (m *Model) refreshStatus() {
	if m.repo == nil {
		return
	}
	ctx := context.Background()
	generation, repository := m.generation, m.repo
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		out := loadResult{kind: resultCommit, generation: generation}
		m.fillChanges(ctx, repository, &out)
		m.publish(&out)
	}()
}

func (m *Model) refreshLog() {
	if m.repo == nil {
		return
	}
	m.startOpen(m.repo.Path)
}

func (m *Model) fillChanges(ctx context.Context, repository *repo.Repo, out *loadResult) {
	status, err := repository.Status(ctx)
	if err != nil && out.err == nil {
		out.err = err
	}
	out.changes = repo.GroupChanges(status.Changes)
	commits, err := repository.Log(ctx, repo.LogQuery{MaxCount: 1})
	if err == nil && len(commits) > 0 {
		out.message = commits[0].Message
	}
}

// mergeInProgress reports a merge git has not finished. The open
// repository's State stays NORMAL until the next snapshot, so the
// commit that concludes a conflicted merge has to look at MERGE_HEAD.
// git commit --only refuses to record that merge and leaves the
// message file behind.
func mergeInProgress(repoPath string) bool {
	_, err := os.Stat(filepath.Join(repoPath, ".git", "MERGE_HEAD"))
	return err == nil
}

func untrackedAmong(paths []string, changes []model.FileChange) []string {
	untracked := map[string]bool{}
	for _, change := range changes {
		if change.Status == model.StatusUntracked || change.Worktree == '?' {
			untracked[change.Path] = true
		}
	}
	var out []string
	for _, path := range paths {
		if untracked[path] {
			out = append(out, path)
		}
	}
	return out
}

func stagedPaths(changes []model.FileChange) []string {
	var paths []string
	seen := map[string]bool{}
	for _, change := range changes {
		if change.Index == 0 || change.Index == '.' || change.Index == ' ' || change.Index == '?' || change.Index == '!' {
			continue
		}
		if seen[change.Path] {
			continue
		}
		seen[change.Path] = true
		paths = append(paths, change.Path)
	}
	return paths
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, path := range a {
		seen[path]++
	}
	for _, path := range b {
		seen[path]--
		if seen[path] < 0 {
			return false
		}
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

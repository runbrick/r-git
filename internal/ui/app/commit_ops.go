package app

import (
	"context"

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
		index := ops.Index{Service: service}
		switch req.Kind {
		case commitview.RequestStage:
			out.err = index.Stage(ctx, req.Paths)
			out.toast = "Staged"
		case commitview.RequestUnstage:
			out.err = index.Unstage(ctx, req.Paths)
			out.toast = "Unstaged"
		case commitview.RequestDiscard:
			out.err = index.Discard(ctx, req.Paths)
			out.toast = "Discarded"
		case commitview.RequestCommit:
			// Stage the checked paths, then unstage the rest, so an
			// unversioned file can be committed and an unchecked staged
			// file stays out. git commit --only refuses an untracked path.
			if err := index.Stage(ctx, req.Paths); err != nil {
				out.err = err
				break
			}
			status, statusErr := repository.Status(ctx)
			if statusErr != nil {
				out.err = statusErr
				break
			}
			if err := index.Unstage(ctx, excluded(req.Paths, changePaths(status.Changes))); err != nil {
				out.err = err
				break
			}
			hash, err := service.Commit(ctx, ops.CommitOptions{
				Message: []byte(req.Message),
				Amend:   req.Amend,
				SignOff: req.SignOff,
				Author:  req.Author,
			})
			out.err = err
			if err == nil {
				out.committed = true
				out.toast = hash.Short()
				if req.Push && m.OnPush != nil {
					m.OnPush()
				}
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

func excluded(included, all []string) []string {
	want := map[string]bool{}
	for _, path := range included {
		want[path] = true
	}
	var out []string
	for _, path := range all {
		if !want[path] {
			out = append(out, path)
		}
	}
	return out
}

func changePaths(changes []model.FileChange) []string {
	var paths []string
	seen := map[string]bool{}
	for _, change := range changes {
		if change.Status == model.StatusIgnored || seen[change.Path] {
			continue
		}
		seen[change.Path] = true
		paths = append(paths, change.Path)
	}
	return paths
}

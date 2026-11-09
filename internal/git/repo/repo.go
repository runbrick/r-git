// Package repo reads repository metadata and history through Git.
package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
)

var (
	ErrNotRepository  = errors.New("not a git repository")
	ErrBareRepository = errors.New("bare repositories are not supported")
)

// Repo is an explicitly-read view of a working tree. Use Snapshot to refresh it;
// callers own refresh timing. Watcher integration is a future TODO.
type Repo struct {
	Path          string
	GitDir        string
	CommonDir     string
	Executor      command.Executor
	State         model.RepoState
	HEAD          model.Hash
	CurrentBranch model.Ref
	Branches      []Reference
	Tags          []Reference
	Remotes       []model.Remote
	Tracking      []model.TrackInfo
}

// Reference is a ref and its resolved object id.
type Reference struct {
	Ref  model.Ref
	Hash model.Hash
}

// Snapshot is an immutable copy of repository metadata and current worktree state.
type Snapshot struct {
	Path          string
	State         model.RepoState
	HEAD          model.Hash
	CurrentBranch model.Ref
	Branches      []Reference
	Tags          []Reference
	Remotes       []model.Remote
	Tracking      []model.TrackInfo
	Changes       []model.FileChange
	Ahead, Behind int
}

// Open reads the repository containing path with the default Git runner.
func Open(ctx context.Context, path string) (*Repo, error) {
	executor, err := command.New()
	if err != nil {
		return nil, err
	}
	return OpenWithExecutor(ctx, path, executor)
}

// OpenWithExecutor reads the repository using an injected command executor.
func OpenWithExecutor(ctx context.Context, path string, executor command.Executor) (*Repo, error) {
	if executor == nil {
		return nil, errors.New("nil git executor")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	r := &Repo{Path: filepath.Clean(abs), Executor: executor}
	bare, err := r.run(ctx, "rev-parse", "--is-bare-repository")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotRepository, path)
	}
	if strings.TrimSpace(string(bare.Stdout)) == "true" {
		return nil, ErrBareRepository
	}
	root, err := r.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotRepository, path)
	}
	r.Path = filepath.Clean(strings.TrimSpace(string(root.Stdout)))
	gitDir, err := r.run(ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	r.GitDir = filepath.Clean(strings.TrimSpace(string(gitDir.Stdout)))
	common, err := r.run(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	r.CommonDir = filepath.Clean(stringValuePath(r.Path, strings.TrimSpace(string(common.Stdout))))
	if err := r.readMetadata(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

func stringValuePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func (r *Repo) run(ctx context.Context, subcommand string, args ...string) (command.Result, error) {
	return r.Executor.Run(ctx, command.Cmd{Dir: r.Path, Subcommand: subcommand, Args: args, ReadOnly: true, NoOptionalLocks: true})
}

func (r *Repo) readMetadata(ctx context.Context) error {
	r.CurrentBranch = model.Ref{}
	head, err := os.ReadFile(filepath.Join(r.GitDir, "HEAD"))
	if err != nil {
		return err
	}
	headText := strings.TrimSpace(string(head))
	r.HEAD = model.Hash{}
	if strings.HasPrefix(headText, "ref: refs/heads/") {
		r.CurrentBranch, _ = model.LocalBranch(strings.TrimPrefix(headText, "ref: "))
	}
	resolved, resolveErr := r.run(ctx, "rev-parse", "--verify", "HEAD^{commit}")
	if resolveErr == nil {
		r.HEAD, err = model.ParseHash(strings.TrimSpace(string(resolved.Stdout)))
		if err != nil {
			return err
		}
	}
	r.Remotes, err = r.readRemotes(ctx)
	if err != nil {
		return err
	}
	refs, err := r.run(ctx, "for-each-ref", "--format=%(refname)%00%(objectname)", "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return err
	}
	r.Branches, r.Tags = nil, nil
	for _, line := range strings.Split(strings.TrimSuffix(string(refs.Stdout), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\x00", 2)
		if len(fields) != 2 {
			continue
		}
		ref, e := model.ParseRef(fields[0])
		if e != nil {
			continue
		}
		h, e := model.ParseHash(fields[1])
		if e != nil {
			continue
		}
		entry := Reference{Ref: ref, Hash: h}
		switch ref.Kind() {
		case model.KindLocalBranch, model.KindRemoteBranch:
			if ref.Kind() == model.KindRemoteBranch && strings.HasSuffix(ref.Name(), "/HEAD") {
				continue
			}
			r.Branches = append(r.Branches, entry)
		case model.KindTag:
			r.Tags = append(r.Tags, entry)
		}
	}
	if r.CurrentBranch.IsZero() && strings.HasPrefix(headText, "ref: ") {
		r.State = model.StateDetached
	} else {
		r.State = model.StateNormal
	}
	if _, e := os.Stat(filepath.Join(r.GitDir, "MERGE_HEAD")); e == nil {
		r.State = model.StateMerging
	}
	if exists(filepath.Join(r.GitDir, "rebase-apply")) || exists(filepath.Join(r.GitDir, "rebase-merge")) {
		r.State = model.StateRebasing
	}
	if r.State != model.StateMerging && r.State != model.StateRebasing {
		switch {
		case r.CurrentBranch.IsZero():
			r.State = model.StateDetached
		case exists(filepath.Join(r.GitDir, "CHERRY_PICK_HEAD")):
			r.State = model.StateGrafting
		case exists(filepath.Join(r.GitDir, "REVERT_HEAD")):
			r.State = model.StateReverting
		}
		if r.State == model.StateNormal {
			todo, e := os.ReadFile(filepath.Join(r.GitDir, "sequencer", "todo"))
			if e == nil {
				if strings.HasPrefix(string(todo), "pick") {
					r.State = model.StateGrafting
				} else if strings.HasPrefix(string(todo), "revert") {
					r.State = model.StateReverting
				}
			}
		}
	}
	if r.State == model.StateRebasing {
		for _, dir := range []string{"rebase-apply", "rebase-merge"} {
			b, e := os.ReadFile(filepath.Join(r.GitDir, dir, "head-name"))
			if e == nil {
				n := strings.TrimSpace(string(b))
				if n != "" && n != "detached HEAD" {
					r.CurrentBranch, _ = model.LocalBranch(n)
				}
				break
			}
		}
	}
	r.Tracking, err = r.readTracking(ctx)
	return err
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func (r *Repo) readRemotes(ctx context.Context) ([]model.Remote, error) {
	out, err := r.run(ctx, "config", "--null", "--get-regexp", "^remote\\..*\\.(url|pushurl|fetch|push)$")
	if err != nil {
		var exit *command.ExitError
		if errors.As(err, &exit) && exit.Code == 1 {
			return nil, nil
		}
		return nil, err
	}
	values := map[string]*model.Remote{}
	for _, item := range strings.Split(string(out.Stdout), "\x00") {
		if item == "" {
			continue
		}
		key, value, ok := strings.Cut(item, "\n")
		if !ok {
			continue
		}
		parts := strings.Split(key, ".")
		if len(parts) < 3 {
			continue
		}
		name := strings.Join(parts[1:len(parts)-1], ".")
		field := parts[len(parts)-1]
		remote := values[name]
		if remote == nil {
			remote = &model.Remote{Name: name}
			values[name] = remote
		}
		switch field {
		case "url":
			remote.URLs = append(remote.URLs, value)
		case "pushurl":
			remote.PushURLs = append(remote.PushURLs, value)
		case "fetch":
			remote.FetchRefSpecs = append(remote.FetchRefSpecs, value)
		case "push":
			remote.PushRefSpecs = append(remote.PushRefSpecs, value)
		}
	}
	result := make([]model.Remote, 0, len(values))
	for _, v := range values {
		if len(v.PushURLs) == 0 {
			v.PushURLs = append([]string(nil), v.URLs...)
		}
		result = append(result, *v)
	}
	return result, nil
}

func (r *Repo) readTracking(ctx context.Context) ([]model.TrackInfo, error) {
	var tracks []model.TrackInfo
	for _, branch := range r.Branches {
		if branch.Ref.Kind() != model.KindLocalBranch {
			continue
		}
		name := branch.Ref.Name()
		remoteOut, e := r.run(ctx, "config", "--get", "branch."+name+".remote")
		if e != nil {
			continue
		}
		mergeOut, e := r.run(ctx, "config", "--get", "branch."+name+".merge")
		if e != nil {
			continue
		}
		merge := true
		if rebase, e := r.run(ctx, "config", "--bool", "--get", "branch."+name+".rebase"); e == nil && strings.TrimSpace(string(rebase.Stdout)) == "true" {
			merge = false
		}
		local, _ := model.LocalBranch(name)
		remote := model.ParseRemoteBranch(strings.TrimPrefix(strings.TrimSpace(string(remoteOut.Stdout)), "."), r.Remotes)
		remoteName := strings.TrimSpace(string(remoteOut.Stdout))
		mergeName := strings.TrimPrefix(strings.TrimSpace(string(mergeOut.Stdout)), "refs/heads/")
		if remoteName != "." {
			remote = model.ParseRemoteBranch(remoteName+"/"+mergeName, r.Remotes)
		} else {
			remote = model.ParseRemoteBranch(mergeName, r.Remotes)
		}
		tracks = append(tracks, model.TrackInfo{Local: local, Remote: remote, Merge: merge})
	}
	return tracks, nil
}

// Snapshot reads fresh status along with the metadata loaded by Open.
func (r *Repo) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := r.readMetadata(ctx); err != nil {
		return Snapshot{}, err
	}
	status, err := r.Status(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Path: r.Path, State: r.State, HEAD: r.HEAD, CurrentBranch: r.CurrentBranch, Branches: append([]Reference(nil), r.Branches...), Tags: append([]Reference(nil), r.Tags...), Remotes: append([]model.Remote(nil), r.Remotes...), Tracking: append([]model.TrackInfo(nil), r.Tracking...), Changes: status.Changes, Ahead: status.Ahead, Behind: status.Behind}, nil
}

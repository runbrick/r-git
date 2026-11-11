package repo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/runbrick/r-git/internal/git/model"
)

// LogQuery controls a page of history. Range takes precedence over Branch,
// which takes precedence over All. With none set, history starts at HEAD.
type LogQuery struct {
	MaxCount, Skip int
	Range, Branch  string
	Paths          []string
	All            bool
}

// CommitParents preserves git log's topological display order while mapping
// each commit to its parent hashes.
type CommitParents struct {
	Commit  model.Hash
	Parents []model.Hash
}

// Log reads a page of commits using NUL-delimited fields, following the
// field-oriented record scheme used by GitLogParser without ambiguous paths.
func (r *Repo) Log(ctx context.Context, q LogQuery) ([]model.Commit, error) {
	// Metadata pages must follow the same order as ParentsOrdered: graph cells
	// connect adjacent topological rows, not independently date-sorted commits.
	args := []string{"log", "--topo-order", "--no-color", "--no-show-signature", "--date=raw", "--format=%x00%H%x00%P%x00%an%x00%ae%x00%at %ai%x00%cn%x00%ce%x00%ct %ci%x00%B%x00"}
	if q.MaxCount > 0 {
		args = append(args, "-n", strconv.Itoa(q.MaxCount))
	}
	if q.Skip > 0 {
		args = append(args, "--skip", strconv.Itoa(q.Skip))
	}
	switch {
	case q.Range != "":
		args = append(args, q.Range)
	case q.Branch != "":
		args = append(args, q.Branch)
	case q.All:
		args = append(args, "--all")
	}
	if len(q.Paths) > 0 {
		args = append(args, "--")
		args = append(args, q.Paths...)
	}
	out, err := r.run(ctx, args[0], args[1:]...)
	if err != nil {
		return nil, err
	}
	return ParseLog(out.Stdout)
}

// ParseLog parses the field records emitted by Log.
func ParseLog(data []byte) ([]model.Commit, error) {
	text := string(data)
	text = strings.Trim(text, "\x00\n")
	if text == "" {
		return nil, nil
	}
	fields := strings.Split(text, "\x00")
	var result []model.Commit
	for len(fields) >= 9 {
		fields = fields[skipEmpty(fields):]
		if len(fields) < 9 {
			break
		}
		hash, err := model.ParseHash(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, err
		}
		var parents []model.Hash
		for _, p := range strings.Fields(fields[1]) {
			h, e := model.ParseHash(p)
			if e != nil {
				return nil, e
			}
			parents = append(parents, h)
		}
		author, err := parseSignature(fields[2], fields[3], fields[4])
		if err != nil {
			return nil, err
		}
		committer, err := parseSignature(fields[5], fields[6], fields[7])
		if err != nil {
			return nil, err
		}
		message := strings.Trim(fields[8], "\n")
		result = append(result, model.Commit{Hash: hash, Parents: parents, Author: author, Committer: committer, Subject: model.SubjectOf(message), Message: message})
		fields = fields[9:]
	}
	return result, nil
}

func skipEmpty(fields []string) int {
	i := 0
	for i < len(fields) && strings.TrimSpace(fields[i]) == "" {
		i++
	}
	return i
}
func parseSignature(name, email, rawTime string) (model.Signature, error) {
	parts := strings.SplitN(rawTime, " ", 2)
	if len(parts) == 0 {
		return model.Signature{}, fmt.Errorf("invalid git timestamp %q", rawTime)
	}
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return model.Signature{}, err
	}
	zone := time.UTC
	if len(parts) > 1 {
		if t, e := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(parts[1])); e == nil {
			zone = t.Location()
		}
	}
	return model.Signature{Name: name, Email: email, When: time.Unix(seconds, 0).In(zone)}, nil
}

// Parents returns hashes in rev-list traversal order and their parent hashes.
func (r *Repo) Parents(ctx context.Context, rev string) (map[model.Hash][]model.Hash, error) {
	if rev == "" {
		rev = "HEAD"
	}
	out, err := r.run(ctx, "rev-list", "--parents", rev)
	if err != nil {
		return nil, err
	}
	result := map[model.Hash][]model.Hash{}
	for _, line := range strings.Split(strings.TrimSpace(string(out.Stdout)), "\n") {
		if line == "" {
			continue
		}
		tokens := strings.Fields(line)
		h, e := model.ParseHash(tokens[0])
		if e != nil {
			return nil, e
		}
		for _, p := range tokens[1:] {
			ph, e := model.ParseHash(p)
			if e != nil {
				return nil, e
			}
			result[h] = append(result[h], ph)
		}
	}
	return result, nil
}

// ParentsOrdered returns commits newest first in git log --topo-order order.
// Unlike Parents, this representation retains the order needed by graph views.
func (r *Repo) ParentsOrdered(ctx context.Context, rev string) ([]CommitParents, error) {
	if rev == "" {
		rev = "HEAD"
	}
	return r.parentsOrdered(ctx, rev)
}

// ParentsOrderedAll returns topologically ordered history reachable from all
// refs, using the same revision set as LogQuery.All.
func (r *Repo) ParentsOrderedAll(ctx context.Context) ([]CommitParents, error) {
	return r.parentsOrdered(ctx, "--all")
}

func (r *Repo) parentsOrdered(ctx context.Context, rev string) ([]CommitParents, error) {
	out, err := r.run(ctx, "log", "--topo-order", "--format=%H %P", rev)
	if err != nil {
		return nil, err
	}
	var result []CommitParents
	for _, line := range strings.Split(strings.TrimSpace(string(out.Stdout)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		hash, err := model.ParseHash(fields[0])
		if err != nil {
			return nil, err
		}
		entry := CommitParents{Commit: hash}
		for _, value := range fields[1:] {
			parent, err := model.ParseHash(value)
			if err != nil {
				return nil, err
			}
			entry.Parents = append(entry.Parents, parent)
		}
		result = append(result, entry)
	}
	return result, nil
}

// ChangedFiles returns a commit's name-status changes, including rename origins.
func (r *Repo) ChangedFiles(ctx context.Context, hash model.Hash) ([]model.FileChange, error) {
	out, err := r.run(ctx, "diff-tree", "--root", "--no-commit-id", "--name-status", "-r", "-z", "-M", "-C", "--find-copies-harder", hash.String())
	if err != nil {
		return nil, err
	}
	return ParseNameStatusZ(out.Stdout)
}

// ParseNameStatusZ parses git diff --name-status -z records.
func ParseNameStatusZ(data []byte) ([]model.FileChange, error) {
	tokens := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	var changes []model.FileChange
	for i := 0; i < len(tokens); {
		if tokens[i] == "" {
			i++
			continue
		}
		status, e := model.ParseFileStatus(model.TrimStatusToken(tokens[i]))
		if e != nil {
			return nil, e
		}
		i++
		if i >= len(tokens) {
			return nil, fmt.Errorf("missing path for %s", status)
		}
		if model.NeedsOrigin(status) {
			origin := tokens[i]
			i++
			if i >= len(tokens) {
				return nil, fmt.Errorf("missing destination for %s", status)
			}
			changes = append(changes, model.FileChange{Status: status, OrigPath: origin, Path: tokens[i]})
		} else {
			changes = append(changes, model.FileChange{Status: status, Path: tokens[i]})
		}
		i++
	}
	return changes, nil
}

// ContainingBranches returns all local and remote branch names containing a
// commit, regardless of the currently loaded history page or branch filter.
func (r *Repo) ContainingBranches(ctx context.Context, hash model.Hash) ([]string, error) {
	out, err := r.run(ctx, "for-each-ref", "--contains="+hash.String(), "--format=%(refname:short)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(out.Stdout))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Conflict struct {
	Path               string
	Base, Ours, Theirs bool
}

func (o Operations) Conflicts(ctx context.Context) ([]Conflict, error) {
	res, err := o.run(ctx, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil, err
	}
	var conflicts []Conflict
	for _, path := range strings.Split(string(res.Stdout), "\x00") {
		if path == "" {
			continue
		}
		c := Conflict{Path: path}
		for _, stage := range []struct {
			n    int
			flag *bool
		}{{1, &c.Base}, {2, &c.Ours}, {3, &c.Theirs}} {
			if _, e := o.run(ctx, "cat-file", "-e", fmt.Sprintf(":%d:%s", stage.n, path)); e == nil {
				*stage.flag = true
			}
		}
		conflicts = append(conflicts, c)
	}
	return conflicts, nil
}

func (o Operations) ReadVersions(ctx context.Context, path string) (base, ours, theirs []byte, err error) {
	values := []*[]byte{&base, &ours, &theirs}
	for i := 1; i <= 3; i++ {
		res, e := o.run(ctx, "show", fmt.Sprintf(":%d:%s", i, path))
		if e != nil {
			continue
		}
		*values[i-1] = res.Stdout
	}
	return base, ours, theirs, nil
}

func (o Operations) Resolve(ctx context.Context, path string, content []byte) error {
	if err := writeWorktreeFile(o.Dir, path, content); err != nil {
		return err
	}
	_, err := o.run(ctx, "add", "--", path)
	return err
}
func (o Operations) AcceptOurs(ctx context.Context, path string) error {
	stage := 2
	if o.isRebasing(ctx) {
		stage = 3
	}
	return o.accept(ctx, path, stage)
}
func (o Operations) AcceptTheirs(ctx context.Context, path string) error {
	stage := 3
	if o.isRebasing(ctx) {
		stage = 2
	}
	return o.accept(ctx, path, stage)
}
func (o Operations) accept(ctx context.Context, path string, stage int) error {
	res, err := o.run(ctx, "show", fmt.Sprintf(":%d:%s", stage, path))
	if err != nil {
		return err
	}
	return o.Resolve(ctx, path, res.Stdout)
}

func (o Operations) isRebasing(ctx context.Context) bool {
	for _, dir := range []string{"rebase-merge", "rebase-apply"} {
		res, err := o.run(ctx, "rev-parse", "--path-format=absolute", "--git-path", dir)
		if err == nil {
			path := strings.TrimSpace(string(res.Stdout))
			if !filepath.IsAbs(path) {
				path = filepath.Join(o.Dir, path)
			}
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				return true
			}
		}
	}
	return false
}

func writeWorktreeFile(root, path string, content []byte) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	full := filepath.Join(rootAbs, filepath.Clean(filepath.FromSlash(path)))
	rel, err := filepath.Rel(rootAbs, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("conflict path escapes worktree: %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return os.WriteFile(full, content, 0644)
}

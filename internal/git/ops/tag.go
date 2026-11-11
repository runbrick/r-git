package ops

import (
	"context"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
)

// CreateTag creates name at revision (git tag <name> <revision>).
// An empty revision tags HEAD, matching GitImpl.createNewTag when the
// reference is empty. The tag hash (the tagged commit) is returned.
func (t *Service) CreateTag(ctx context.Context, name, revision string) (string, error) {
	args := []string{name}
	if revision != "" {
		args = append(args, revision)
	}
	if _, err := t.run(ctx, "tag", args...); err != nil {
		return "", err
	}
	ref := "refs/tags/" + name
	res, err := t.Git.Run(ctx, command.Cmd{
		Dir:        t.Root,
		Subcommand: "rev-parse",
		Args:       []string{ref + "^{commit}"},
		ReadOnly:   true,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

// DeleteTag removes the tag name (git tag -d <name>).
func (t *Service) DeleteTag(ctx context.Context, name string) error {
	_, err := t.run(ctx, "tag", "-d", name)
	return err
}

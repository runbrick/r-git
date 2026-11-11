package ops

import "context"

// Stage adds paths to the index, including new files.
// An empty path list is a no-op, matching GitFileUtils.executeForPaths.
// The arguments follow GitAddOperation's forced GitFileUtils.addPaths:
// --ignore-errors -A -f.
func (ix *Service) Stage(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"--ignore-errors", "-A", "-f", "--"}, paths...)
	_, err := ix.run(ctx, "add", args...)
	return err
}

// Unstage removes paths from the index without changing the work tree
// (git reset -- <paths>). An empty path list is a no-op.
func (ix *Service) Unstage(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"--"}, paths...)
	_, err := ix.run(ctx, "reset", args...)
	return err
}

// Discard drops uncommitted changes to paths.
//
// A path that is tracked in HEAD is restored in both the index and the
// work tree (git checkout -- <path>), which undoes staged and unstaged
// edits and restores a deleted tracked file. A path that is not in HEAD
// (untracked, or added and not yet committed) is removed from the index
// and the work tree. Ignored files are not removed.
func (ix *Service) Discard(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	var tracked, untracked []string
	for _, p := range paths {
		if ix.trackedInHead(ctx, p) {
			tracked = append(tracked, p)
		} else if !ix.ignored(ctx, p) {
			untracked = append(untracked, p)
		}
	}
	if len(tracked) > 0 {
		args := append([]string{"--"}, tracked...)
		if _, err := ix.run(ctx, "checkout", args...); err != nil {
			return err
		}
	}
	if len(untracked) > 0 {
		// Drop an added-but-uncommitted path from the index. A purely
		// untracked path is not in the index; git rm --cached reports that
		// and the work-tree removal below still applies.
		args := append([]string{"--ignore-unmatch", "--cached", "-r", "--"}, untracked...)
		if _, err := ix.run(ctx, "rm", args...); err != nil && !ix.untrackedOnly(ctx, untracked) {
			return err
		}
		args = []string{"-f", "--"}
		args = append(args, untracked...)
		if _, err := ix.run(ctx, "clean", args...); err != nil {
			return err
		}
	}
	return nil
}

func (ix *Service) trackedInHead(ctx context.Context, path string) bool {
	_, err := ix.Git.Run(ctx, cmdRead(ix.Root, "cat-file", "-e", "HEAD:"+path))
	return err == nil
}

func (ix *Service) ignored(ctx context.Context, path string) bool {
	res, err := ix.Git.Run(ctx, cmdRead(ix.Root, "check-ignore", "--", path))
	return err == nil && len(res.Stdout) > 0
}

func (ix *Service) untrackedOnly(ctx context.Context, paths []string) bool {
	for _, p := range paths {
		res, err := ix.Git.Run(ctx, cmdRead(ix.Root, "ls-files", "--error-unmatch", "--", p))
		if err == nil && len(res.Stdout) > 0 {
			return false
		}
	}
	return true
}

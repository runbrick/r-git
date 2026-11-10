package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
)

// ErrUnmergedFiles is returned when checkout, cherry-pick, or revert cannot
// proceed because the index has unmerged entries.
// Java: GitSimpleEventDetector.Event.UNMERGED_PREVENTING_CHECKOUT.
var ErrUnmergedFiles = errors.New("unmerged files prevent the operation")

// ErrEmptyCommit is returned when commit, cherry-pick, or revert has
// nothing to record. Java maps the "nothing to commit" family of messages
// to a dedicated empty-commit error.
var ErrEmptyCommit = errors.New("nothing to commit")

// ErrConflict is returned when cherry-pick, revert, or unstash stops on a
// content conflict. The working tree is left in the conflicted state.
var ErrConflict = errors.New("merge conflict")

// ErrAlreadyApplied is returned when cherry-pick reports that the change
// is already present (an empty cherry-pick).
var ErrAlreadyApplied = errors.New("commit already applied")

// ErrNotFullyMerged is returned by Branches.Delete when git refuses to
// delete a branch that is not fully merged and Force is false.
// Commits lists the commits reachable from the branch and not from Base
// (git log --format=%H Base..name). Base is the branch git named in the
// warning, or HEAD when git only said the branch is not fully merged.
type ErrNotFullyMerged struct {
	Branch  string
	Base    string
	Commits []model.Hash
}

func (e *ErrNotFullyMerged) Error() string {
	return fmt.Sprintf("branch %q is not fully merged to %q (%d unmerged commits)", e.Branch, e.Base, len(e.Commits))
}

// ErrGPGSign is returned when commit fails because gpg could not sign.
var ErrGPGSign = errors.New("gpg failed to sign the data")

// classify turns a failed git invocation into a typed error when the
// output matches a detector Java uses. Otherwise it returns the original
// error, which is a *command.ExitError.
func classify(res command.Result, err error, op command.OverwriteOperation) error {
	if err == nil {
		return nil
	}
	lines := append(append([]string{}, res.ErrorLines...), res.OutputLines...)
	if detected := command.DetectLocalChangesWouldBeOverwritten(lines, op); detected != nil {
		return detected
	}
	if detected := command.DetectUntrackedFilesWouldBeOverwritten(lines); detected != nil {
		return detected
	}
	if detected := command.DetectBranchCheckedOutInWorktree(lines); detected != nil {
		return detected
	}
	if command.DetectSimpleEvent(lines, command.UnmergedPreventingCheckout) ||
		command.DetectSimpleEvent(lines, command.UnmergedPreventingMerge) {
		return fmt.Errorf("%w", ErrUnmergedFiles)
	}
	if command.DetectSimpleEvent(lines, command.CherryPickConflict) ||
		command.DetectSimpleEvent(lines, command.MergeConflict) ||
		command.DetectSimpleEvent(lines, command.MergeConflictOnUnstash) ||
		command.DetectSimpleEvent(lines, command.IndexConflictOnUnstash) {
		return fmt.Errorf("%w: %s", ErrConflict, strings.TrimSpace(string(res.Stderr)))
	}
	if command.DetectSimpleEvent(lines, command.LocalChangesOverwrittenByCherryPick) {
		if detected := command.DetectLocalChangesWouldBeOverwritten(lines, command.Merge); detected != nil {
			return detected
		}
		return &command.ErrLocalChangesWouldBeOverwritten{}
	}
	if isEmptyCommit(res) {
		return fmt.Errorf("%w", ErrEmptyCommit)
	}
	if containsFold(string(res.Stderr), "gpg failed to sign the data") {
		return fmt.Errorf("%w", ErrGPGSign)
	}
	return err
}

func isEmptyCommit(res command.Result) bool {
	text := strings.ToLower(string(res.Stdout) + "\n" + string(res.Stderr))
	for _, needle := range []string{
		"no changes",
		"no changes added to commit",
		"nothing added to commit",
		"nothing to commit",
	} {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func containsFold(s, needle string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(needle))
}

// head returns the full hash of HEAD, or the zero hash when the
// repository has no commits.
func (s *Service) head(ctx context.Context) (model.Hash, error) {
	res, err := s.Git.Run(ctx, command.Cmd{
		Dir:        s.Root,
		Subcommand: "rev-parse",
		Args:       []string{"--verify", "HEAD"},
		ReadOnly:   true,
	})
	if err != nil {
		var exit *command.ExitError
		if errors.As(err, &exit) {
			return model.Hash{}, nil
		}
		return model.Hash{}, err
	}
	return model.ParseHash(strings.TrimSpace(string(res.Stdout)))
}

// currentBranch returns the short name of the current branch, or "" when
// HEAD is detached or the repository has no commits.
func (s *Service) currentBranch(ctx context.Context) (string, error) {
	res, err := s.Git.Run(ctx, command.Cmd{
		Dir:        s.Root,
		Subcommand: "symbolic-ref",
		Args:       []string{"--short", "HEAD"},
		ReadOnly:   true,
	})
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

// logHashes returns commit hashes for a rev-list range, newest first.
func (s *Service) logHashes(ctx context.Context, rangeExpr string) ([]model.Hash, error) {
	res, err := s.Git.Run(ctx, command.Cmd{
		Dir:        s.Root,
		Subcommand: "log",
		Args:       []string{"--format=%H", rangeExpr},
		ReadOnly:   true,
	})
	if err != nil {
		return nil, err
	}
	var hashes []model.Hash
	for _, line := range strings.Split(strings.TrimSpace(string(res.Stdout)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		h, err := model.ParseHash(line)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, nil
}

// writeMessageFile writes message bytes to a temp file next to the
// repository and returns its path. The caller deletes the file.
// Java writes git-commit-msg-*.txt and passes it with -F so the message
// bytes are not re-encoded by the command line.
func writeMessageFile(dir string, message []byte) (string, error) {
	f, err := os.CreateTemp(dir, "git-commit-msg-*.txt")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.Write(message); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// messageFileArgs writes message to a temp file under the repository and
// returns the -F arguments. The caller must remove the returned path.
func (s *Service) messageFileArgs(message []byte) (args []string, path string, err error) {
	path, err = writeMessageFile(s.Root, message)
	if err != nil {
		return nil, "", err
	}
	return []string{"-F", path}, path, nil
}

func removeFile(path string) {
	if path != "" {
		os.Remove(path)
	}
}

// preservingMessage matches the stash subject Java's GitPreservingProcess
// builds: "<Operation> at <date time>".
func preservingMessage(operation string, now time.Time) string {
	if operation == "" {
		operation = "Operation"
	}
	titled := strings.ToUpper(operation[:1]) + operation[1:]
	return fmt.Sprintf("%s at %s", titled, now.Format("2006-01-02 15:04:05"))
}

// cmdRead builds a read-only command in dir.
func cmdRead(dir, sub string, args ...string) command.Cmd {
	return command.Cmd{Dir: dir, Subcommand: sub, Args: args, ReadOnly: true}
}

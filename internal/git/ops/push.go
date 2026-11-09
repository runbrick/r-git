package ops

import (
	"context"
	"regexp"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
)

const (
	NoFFRejectReason = "non-fast-forward"
	FetchFirstReason = "fetch first"
	StaleInfoReason  = "stale info"
	FailedLockReason = "failed to lock"
)

type PushNativeResultType int

const (
	PushSuccess PushNativeResultType = iota
	PushForcedUpdate
	PushNewRef
	PushRejected
	PushDeleted
	PushUpToDate
	PushError
)

type PushNativeResult struct {
	Type      PushNativeResultType
	SourceRef string
	Reason    string
	Range     string
}

func (r PushNativeResult) IsNonFFUpdate() bool {
	if r.Reason == "" || r.Type != PushRejected {
		return false
	}
	reasonLower := strings.ToLower(r.Reason)
	return strings.Contains(reasonLower, NoFFRejectReason) ||
		strings.Contains(reasonLower, FetchFirstReason) ||
		strings.Contains(reasonLower, FailedLockReason)
}

func (r PushNativeResult) IsStaleInfo() bool {
	return r.Type == PushRejected && r.Reason != "" && strings.Contains(strings.ToLower(r.Reason), StaleInfoReason)
}

var (
	pushPattern  = regexp.MustCompile(`^.*([ \+\-\*!=])\t(\S+):(\S+)\t([^\(]+)(?:\((.+)\))?.*$`)
	rangePattern = regexp.MustCompile(`^[0-9a-f]+[\.]{2,3}[0-9a-f]+$`)
)

func ParsePushNativeResult(output []string) []PushNativeResult {
	var results []PushNativeResult
	for _, line := range output {
		matches := pushPattern.FindStringSubmatch(line)
		if matches != nil {
			result := parseRefResult(matches)
			if result != nil {
				results = append(results, *result)
			}
		}
	}
	return results
}

func parseRefResult(matches []string) *PushNativeResult {
	flag := matches[1]
	from := matches[2]
	summary := strings.TrimSpace(matches[4])
	reason := ""
	if len(matches) > 5 {
		reason = matches[5]
	}

	typ, ok := parseType(flag)
	if !ok {
		return nil
	}

	rng := ""
	if rangePattern.MatchString(summary) {
		rng = summary
	}

	return &PushNativeResult{
		Type:      typ,
		SourceRef: from,
		Reason:    reason,
		Range:     rng,
	}
}

func parseType(flag string) (PushNativeResultType, bool) {
	switch flag[0] {
	case ' ':
		return PushSuccess, true
	case '+':
		return PushForcedUpdate, true
	case '-':
		return PushDeleted, true
	case '*':
		return PushNewRef, true
	case '!':
		return PushRejected, true
	case '=':
		return PushUpToDate, true
	default:
		return PushError, false
	}
}

type Pusher struct {
	runner *command.Runner
}

func NewPusher(runner *command.Runner) *Pusher {
	return &Pusher{runner: runner}
}

// Push executes a git push operation.
func (p *Pusher) Push(ctx context.Context, repoPath, remote string, refspec string, force bool) ([]PushNativeResult, error) {
	args := []string{"--porcelain"}
	if force {
		args = append(args, "--force-with-lease")
	}
	args = append(args, remote, refspec)

	cmd := command.Cmd{
		Subcommand: "push",
		Args:       args,
		Dir:        repoPath,
	}

	res, err := p.runner.Run(ctx, cmd)
	if err != nil {
		if _, ok := err.(*command.ExitError); !ok {
			return nil, err
		}
	}

	lines := strings.Split(string(res.Stdout), "\n")
	return ParsePushNativeResult(lines), nil
}

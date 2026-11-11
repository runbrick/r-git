package ops

import (
	"context"
	"errors"
	"fmt"
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

// Push executes a git push operation. setUpstream sets the local branch's
// upstream to the pushed remote branch when the refspec is a branch mapping.
// A non-zero exit with ref results is represented by those results (including
// rejections); without ref results, the returned error wraps command.ExitError.
func (p *Service) Push(ctx context.Context, remote string, refspec string, force, setUpstream bool) ([]PushNativeResult, error) {
	args := []string{"--porcelain"}
	if force {
		args = append(args, "--force-with-lease")
	}
	if setUpstream {
		args = append(args, "--set-upstream")
	}
	args = append(args, remote, refspec)

	res, err := p.run(ctx, "push", args...)
	if err != nil {
		var exitErr *command.ExitError
		if !errors.As(err, &exitErr) {
			return nil, err
		}
	}

	lines := strings.Split(string(res.Stdout), "\n")
	results := ParsePushNativeResult(lines)
	if err != nil && len(results) == 0 {
		return nil, fmt.Errorf("push to %q: %w", remote, err)
	}
	return results, nil
}

// Ensure PushNativeResultType matches Java enums
func PushResultToString(t PushNativeResultType) string {
	switch t {
	case PushSuccess:
		return "SUCCESS"
	case PushForcedUpdate:
		return "FORCED_UPDATE"
	case PushNewRef:
		return "NEW_REF"
	case PushRejected:
		return "REJECTED"
	case PushDeleted:
		return "DELETED"
	case PushUpToDate:
		return "UP_TO_DATE"
	default:
		return "ERROR"
	}
}

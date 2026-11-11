package ops_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
)

type serviceExecutor struct {
	run    func(context.Context, command.Cmd) (command.Result, error)
	stream func(context.Context, command.Cmd, func(command.Line)) (command.Result, error)
}

func (e serviceExecutor) Run(ctx context.Context, cmd command.Cmd) (command.Result, error) {
	return e.run(ctx, cmd)
}

func (e serviceExecutor) Stream(ctx context.Context, cmd command.Cmd, progress func(command.Line)) (command.Result, error) {
	return e.stream(ctx, cmd, progress)
}

// Remote operations must work through the same fakeable executor and repository
// binding as local operations, without a concrete Runner or per-call repo path.
func TestServiceRemoteRepositoryAndErrors(t *testing.T) {
	failed := errors.New("executor failed")
	for _, tt := range []struct {
		name string
		run  func(context.Context, *ops.Service) error
	}{
		{"fetch", func(ctx context.Context, s *ops.Service) error {
			return s.Fetch(ctx, ops.FetchParams{Remotes: []string{"origin"}})
		}},
		{"pull", func(ctx context.Context, s *ops.Service) error { return s.Pull(ctx, ops.PullParams{}) }},
		{"push", func(ctx context.Context, s *ops.Service) error {
			_, err := s.Push(ctx, "origin", "main:main", false, false)
			return err
		}},
		{"add remote", func(ctx context.Context, s *ops.Service) error { return s.AddRemote(ctx, "origin", "url") }},
		{"remove remote", func(ctx context.Context, s *ops.Service) error { return s.RemoveRemote(ctx, "origin") }},
		{"rename remote", func(ctx context.Context, s *ops.Service) error { return s.RenameRemote(ctx, "origin", "upstream") }},
		{"set remote URL", func(ctx context.Context, s *ops.Service) error { return s.SetRemoteURL(ctx, "origin", "url") }},
		{"stash for pull", func(ctx context.Context, s *ops.Service) error { return s.Stash(ctx, "saved") }},
		{"unstash after pull", func(ctx context.Context, s *ops.Service) error { return s.Unstash(ctx) }},
		{"stash list", func(ctx context.Context, s *ops.Service) error {
			_, err := s.StashList(ctx)
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			s := ops.New(serviceExecutor{run: func(got context.Context, cmd command.Cmd) (command.Result, error) {
				calls++
				if cmd.Dir != "bound-repository" {
					t.Errorf("repository = %q", cmd.Dir)
				}
				cancel()
				if !errors.Is(got.Err(), context.Canceled) {
					t.Error("executor did not receive caller cancellation")
				}
				return command.Result{}, failed
			}}, "bound-repository")
			if err := tt.run(ctx, s); err != failed || calls != 1 {
				t.Fatalf("error = %v, calls = %d", err, calls)
			}
		})
	}
}

func TestServiceRemoteProgress(t *testing.T) {
	progress := command.Line{Text: "Receiving objects: 50%", Stream: command.Stderr, CR: true}
	for _, fetch := range []bool{true, false} {
		t.Run(map[bool]string{true: "fetch", false: "pull"}[fetch], func(t *testing.T) {
			var lines []command.Line
			callback := func(line command.Line) { lines = append(lines, line) }
			s := ops.New(serviceExecutor{stream: func(_ context.Context, cmd command.Cmd, onLine func(command.Line)) (command.Result, error) {
				if cmd.Dir != "bound-repository" {
					t.Errorf("repository = %q", cmd.Dir)
				}
				onLine(progress)
				return command.Result{}, context.Canceled
			}}, "bound-repository")
			var err error
			if fetch {
				err = s.Fetch(context.Background(), ops.FetchParams{Remotes: []string{"origin"}, OnProgress: callback})
			} else {
				err = s.Pull(context.Background(), ops.PullParams{OnProgress: callback})
			}
			if err != context.Canceled || !reflect.DeepEqual(lines, []command.Line{progress}) {
				t.Fatalf("error = %v, progress = %#v", err, lines)
			}
		})
	}
}

func TestServicePushPreservesExitResults(t *testing.T) {
	for _, tt := range []struct {
		name      string
		output    string
		wantError bool
	}{
		{"rejected ref", "!\trefs/heads/main:refs/heads/main\t[rejected] (non-fast-forward)\n", false},
		{"empty stdout", "", true},
		{"no ref results", "To origin\nDone\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exitErr := &command.ExitError{Code: 1, Stderr: "remote unavailable"}
			s := ops.New(serviceExecutor{run: func(context.Context, command.Cmd) (command.Result, error) {
				return command.Result{Stdout: []byte(tt.output)}, exitErr
			}}, "bound-repository")
			results, err := s.Push(context.Background(), "origin", "main:main", false, false)
			if tt.wantError {
				var got *command.ExitError
				if !errors.As(err, &got) || got != exitErr || err == exitErr {
					t.Fatalf("expected wrapped ExitError, got %v", err)
				}
				if !strings.Contains(err.Error(), exitErr.Stderr) || len(results) != 0 {
					t.Fatalf("results = %#v, error = %v", results, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || !results[0].IsNonFFUpdate() {
				t.Fatalf("lost rejection: %#v", results)
			}
		})
	}
}

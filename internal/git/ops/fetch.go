package ops

import (
	"context"
	"regexp"

	"github.com/runbrick/r-git/internal/git/command"
	"golang.org/x/sync/errgroup"
)

var prunePattern = regexp.MustCompile(`\s*x\s*\[deleted\].*->\s*(\S*)`)

type FetchParams struct {
	Remotes    []string
	Refspecs   []string
	Prune      bool
	Tags       bool
	Unshallow  bool
	OnProgress func(command.Line)
}

func (f *Service) Fetch(ctx context.Context, params FetchParams) error {
	if len(params.Remotes) == 0 {
		return nil
	}

	// Fetching several remotes concurrently (Java uses MAX_SSH_CONNECTIONS = 10 logic implicitly)
	// We'll use a simple errgroup with a bounded concurrency.
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(10)

	for _, remote := range params.Remotes {
		remote := remote
		eg.Go(func() error {
			args := []string{"--progress"}
			if params.Prune {
				args = append(args, "--prune")
			}
			if params.Tags {
				args = append(args, "--tags")
			}
			if params.Unshallow {
				args = append(args, "--unshallow")
			}
			args = append(args, remote)
			args = append(args, params.Refspecs...)

			cmd := command.Cmd{
				Subcommand: "fetch",
				Args:       args,
				Dir:        f.Root,
			}

			if params.OnProgress != nil {
				_, err := f.Git.Stream(egCtx, cmd, params.OnProgress)
				return err
			}

			_, err := f.Git.Run(egCtx, cmd)
			return err
		})
	}

	return eg.Wait()
}

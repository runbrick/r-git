package repo

import (
	"context"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/testutil"
)

func TestLogAndParentsOrderedAllIncludeUnmergedBranch(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("shared base")
	mainBranch := strings.TrimSpace(g.Git("branch", "--show-current"))
	g.Git("checkout", "-q", "-b", "topic")
	g.WriteFile("topic.txt", "topic\n")
	topic := g.Commit("topic only")
	g.Git("checkout", "-q", mainBranch)
	g.WriteFile("main.txt", "main\n")
	g.Commit("main only")
	r := openTestRepo(t, g.Dir)
	ctx := context.Background()
	commits, err := r.Log(ctx, LogQuery{All: true})
	if err != nil {
		t.Fatal(err)
	}
	ordered, err := r.ParentsOrderedAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 3 || len(ordered) != 3 {
		t.Fatalf("all refs: metadata=%d topology=%d", len(commits), len(ordered))
	}
	page, err := r.Log(ctx, LogQuery{All: true, Skip: 1, MaxCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || !page[0].Hash.Equal(commits[1].Hash) {
		t.Fatalf("all refs page = %v", page)
	}
	branch, err := r.Log(ctx, LogQuery{All: true, Branch: mainBranch})
	if err != nil {
		t.Fatal(err)
	}
	for _, commit := range branch {
		if commit.Hash.String() == topic {
			t.Fatal("explicit branch included unmerged topic")
		}
	}
}

func TestLogPagesMatchTopologicalGraphOrder(t *testing.T) {
	g := testutil.NewDateSkewedHistory(t)
	r := openTestRepo(t, g.Dir)
	ctx := context.Background()
	for _, tc := range []struct {
		name, rev string
		query     LogQuery
	}{
		{name: "HEAD", rev: "HEAD"},
		{name: "branch", rev: "feature/ui", query: LogQuery{Branch: "feature/ui"}},
		{name: "All", rev: "--all", query: LogQuery{All: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ordered []CommitParents
			var err error
			if tc.query.All {
				ordered, err = r.ParentsOrderedAll(ctx)
			} else {
				ordered, err = r.ParentsOrdered(ctx, tc.query.Branch)
			}
			if err != nil {
				t.Fatal(err)
			}
			dateOrder := strings.Fields(g.Git("log", "--format=%H", tc.rev))
			differs := false
			for i := range ordered {
				differs = differs || dateOrder[i] != ordered[i].Commit.String()
			}
			if !differs {
				t.Fatal("fixture must distinguish date traversal from topological order")
			}
			for _, pageSize := range []int{0, 1, 2, 4} {
				for skip := 0; skip <= len(ordered); skip++ {
					q := tc.query
					q.Skip, q.MaxCount = skip, pageSize
					commits, err := r.Log(ctx, q)
					if err != nil {
						t.Fatal(err)
					}
					want := len(ordered) - skip
					if pageSize > 0 {
						want = min(want, pageSize)
					}
					if len(commits) != want {
						t.Fatalf("skip=%d max=%d: got %d, want %d", skip, pageSize, len(commits), want)
					}
					for i, commit := range commits {
						if !commit.Hash.Equal(ordered[skip+i].Commit) {
							t.Fatalf("skip=%d max=%d row=%d: metadata %s != graph %s", skip, pageSize, i, commit.Hash, ordered[skip+i].Commit)
						}
					}
				}
			}
		})
	}
}

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

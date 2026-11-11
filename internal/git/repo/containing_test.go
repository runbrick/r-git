package repo

import (
	"context"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/testutil"
	"reflect"
	"testing"
)

func TestContainingBranchesIncludesRemoteAndExcludesTags(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base")
	base := g.Commit("base")
	g.Git("branch", "topic")
	g.Git("update-ref", "refs/remotes/origin/topic", base)
	g.Git("tag", "release")
	g.WriteFile("base.txt", "next")
	tip := g.Commit("next")
	r, err := Open(context.Background(), g.Dir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := r.ContainingBranches(context.Background(), model.MustParseHash(base))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("branches %v", names)
	}
	names, err = r.ContainingBranches(context.Background(), model.MustParseHash(tip))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{r.CurrentBranch.Name()}) {
		t.Fatalf("tip branches %v", names)
	}
}

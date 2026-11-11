package log

import (
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
)

func TestFiltersCombineAndRejectInvalidRegex(t *testing.T) {
	now := time.Now()
	commits := []model.Commit{
		{Hash: model.MustParseHash(strings.Repeat("a", 40)), Subject: "Fix graph", Author: model.Signature{Name: "Alice", Email: "alice@example.invalid"}, Committer: model.Signature{When: now}},
		{Hash: model.MustParseHash(strings.Repeat("b", 40)), Subject: "fix layout", Author: model.Signature{Name: "Bob"}, Committer: model.Signature{When: now.AddDate(0, 0, -10)}},
		{Hash: model.MustParseHash(strings.Repeat("c", 40)), Subject: "Document graph", Author: model.Signature{Name: "Alice"}, Committer: model.Signature{When: now.AddDate(0, 0, -40)}},
	}
	for _, tc := range []struct {
		name, filter, user, date string
		regex, matchCase         bool
		want                     int
		invalid                  bool
	}{
		{name: "subject", filter: "fix", want: 2},
		{name: "hash", filter: "bbbbbb", want: 1},
		{name: "case", filter: "Fix", matchCase: true, want: 1},
		{name: "regex", filter: "^(fix|document)", regex: true, want: 3},
		{name: "regex case", filter: "^fix", regex: true, matchCase: true, want: 1},
		{name: "author email", user: "ALICE@", want: 1},
		{name: "combined", filter: "graph", user: "alice", date: "Last 7 days", want: 1},
		{name: "until", date: "Before today", want: 2},
		{name: "since", date: "Last 30 days", want: 2},
		{name: "invalid", filter: "[", regex: true, invalid: true, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := View{Filter: tc.filter, User: tc.user, Date: tc.date, Regex: tc.regex, MatchCase: tc.matchCase}
			v.AppendPage(commits, true)
			if got := v.VisibleCount(); got != tc.want {
				t.Fatalf("count %d want %d", got, tc.want)
			}
			if (v.FilterError != "") != tc.invalid {
				t.Fatalf("error %q", v.FilterError)
			}
		})
	}
}

func TestGraphHeadColorIndexMatchesJava(t *testing.T) {
	colors := themeColors{1: "master", 2: "topic/🚀"}
	for _, tc := range []struct {
		head                      int32
		headIndex, fragment, want int
	}{
		{1, 3, 3, -1081267614}, {2, 4, 4, -955568445}, {1, 3, 7, 7}, {99, 5, 5, 0},
	} {
		if got := colors.GetColor(tc.head, tc.headIndex, tc.fragment); got != tc.want {
			t.Errorf("%+v got %d", tc, got)
		}
	}
}

func TestGraphBestRefAndMergeGeometry(t *testing.T) {
	hash := func(c string) model.Hash { return model.MustParseHash(strings.Repeat(c, 40)) }
	local, _ := model.LocalBranch("feature")
	main, _ := model.LocalBranch("main")
	remote, _ := model.RemoteBranchRef("origin/main")
	ordered := []repo.CommitParents{{Commit: hash("a"), Parents: []model.Hash{hash("b"), hash("c")}}, {Commit: hash("b"), Parents: []model.Hash{hash("c")}}, {Commit: hash("c")}}
	graph, _, err := NewGraphRows(ordered, repo.Reference{Ref: local, Hash: hash("a")}, repo.Reference{Ref: main, Hash: hash("a")}, repo.Reference{Ref: remote, Hash: hash("a")})
	if err != nil {
		t.Fatal(err)
	}
	row := graph.Row(0)
	var expected int32
	for _, r := range "origin/main" {
		expected = 31*expected + int32(r)
	}
	if len(row.Nodes) != 1 || row.Nodes[0].ColorId != int(expected) {
		t.Fatalf("head row %+v", row)
	}
	diagonal := false
	for i := range ordered {
		for _, edge := range graph.Row(i).Edges {
			diagonal = diagonal || edge.PositionInCurrentRow != edge.PositionInOtherRow
		}
	}
	if !diagonal || graph.width < 32 {
		t.Fatalf("merge lanes missing (width %v)", graph.width)
	}
}

func TestShortDate(t *testing.T) {
	when := time.Date(2026, 6, 16, 23, 31, 0, 0, time.FixedZone("+0800", 8*3600))
	if got := shortDate(when); got != "16/6/26, 11:31 pm" {
		t.Fatal(got)
	}
}

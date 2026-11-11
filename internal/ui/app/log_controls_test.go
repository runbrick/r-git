package app

import (
	"reflect"
	"testing"

	"github.com/runbrick/r-git/internal/testutil"
	commitview "github.com/runbrick/r-git/internal/ui/commit"
)

func TestLogFiltersAndDetailsControls(t *testing.T) {
	g := newLogFixture(t)
	m, d := openDriver(t, g)
	waitFor(t, d, func() bool { return len(m.log.Commits) == 2 && len(m.log.Details) > 0 })
	if hasLabel(d, "Hash") {
		t.Fatal("hash column must start hidden")
	}
	clickControl(t, d, "Show details")
	if hasLabel(d, "Changed files") {
		t.Fatal("details toggle did not hide pane")
	}
	clickControl(t, d, "Show details")
	if !hasLabel(d, "Changed files") {
		t.Fatal("details did not return")
	}
	clickControl(t, d, "Log options")
	clickControl(t, d, "Show hash")
	if !hasLabel(d, "Hash") {
		t.Fatal("hash column did not return")
	}
	clickControl(t, d, "Regex")
	clickControl(t, d, "Filter")
	if err := d.Type("^FIX"); err != nil {
		t.Fatal(err)
	}
	if m.log.VisibleCount() != 1 {
		t.Fatalf("regex count %d", m.log.VisibleCount())
	}
	clickControl(t, d, "Match case")
	if m.log.VisibleCount() != 0 {
		t.Fatal("case toggle ignored")
	}
	clickControl(t, d, "Match case")
	clickControl(t, d, "User")
	clickControl(t, d, "User filter")
	if err := d.Type("nobody"); err != nil {
		t.Fatal(err)
	}
	if m.log.VisibleCount() != 0 {
		t.Fatal("author filter ignored")
	}
}

func TestCommitGroupCheckboxesAndAmendMenu(t *testing.T) {
	g := newLogFixture(t)
	m, d := openDriver(t, g)
	waitFor(t, d, func() bool { return len(m.commit.Entries) == 2 })
	clickControl(t, d, "Include Changes")
	var req commitview.Request
	m.commit.OnRequest = func(r commitview.Request) { req = r }
	clickControl(t, d, "Commit message")
	if err := d.Type("record notes"); err != nil {
		t.Fatal(err)
	}
	// Viva follow-up: pinned ui/paint.go:311 paintTextarea ignores
	// flagMono, unlike paintInput at :299. Verify editing and submission here;
	// assert the rendered mono face once Viva supports it (keep .Mono() intent).
	if m.commit.Message != "record notes" || !hasLabel(d, "record notes") {
		t.Fatal("message editor must retain and paint the typed draft")
	}

	clickControl(t, d, "Commit changes")
	if req.Message != "record notes" {
		t.Fatalf("submitted message %q", req.Message)
	}
	if !reflect.DeepEqual(req.Paths, []string{"notes.txt"}) {
		t.Fatalf("checked paths %v", req.Paths)
	}
	clickControl(t, d, "Include Unversioned Files")
	clickControl(t, d, "Include Changes")
	clickControl(t, d, "Commit changes")
	if !reflect.DeepEqual(req.Paths, []string{"base.txt"}) {
		t.Fatalf("checked paths %v", req.Paths)
	}
	clickControl(t, d, "last commit")
	clickControl(t, d, "Amend last commit")
	if !m.commit.Amend {
		t.Fatal("last commit menu did not enable Amend")
	}
}

func newLogFixture(t *testing.T) string {
	t.Helper()
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base")
	g.Commit("Initial")
	g.WriteFile("base.txt", "fix")
	g.Commit("Fix graph")
	g.WriteFile("base.txt", "working")
	g.WriteFile("notes.txt", "notes")
	return g.Dir
}

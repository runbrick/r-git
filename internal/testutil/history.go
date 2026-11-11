package testutil

import "testing"

// NewDateSkewedHistory makes the demo's two feature branches and merge, with a
// login parent dated after its merge. Date traversal and topological order
// therefore disagree even when the git commands run within the same second.
func NewDateSkewedHistory(t *testing.T) *Repo {
	t.Helper()
	g := NewRepo(t)
	g.Git("checkout", "-q", "-b", "main")
	commit := func(path, message, date string) {
		t.Setenv("GIT_AUTHOR_DATE", date)
		t.Setenv("GIT_COMMITTER_DATE", date)
		g.WriteFile(path, message+"\n")
		g.Commit(message)
	}
	commit("base.txt", "Initial commit", "2026-01-01T12:00:00Z")
	g.Git("checkout", "-q", "-b", "feature/login")
	commit("login.go", "Add login", "2026-01-02T12:00:00Z")
	commit("login.go", "Login returns bool", "2026-01-05T12:00:00Z")
	g.Git("checkout", "-q", "main")
	commit("readme.txt", "Describe the repo", "2026-01-03T12:00:00Z")
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-04T12:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-04T12:00:00Z")
	g.Git("merge", "--no-ff", "-m", "Merge feature/login", "feature/login")
	g.Git("checkout", "-q", "-b", "feature/ui")
	commit("ui.txt", "Start UI work", "2026-01-06T12:00:00Z")
	g.Git("checkout", "-q", "main")
	return g
}

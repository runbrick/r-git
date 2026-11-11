package model

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCommitRoundTrip(t *testing.T) {
	when := time.Date(2024, 3, 1, 12, 0, 0, 0, time.FixedZone("CET", 3600))
	parent := MustParseHash(strings.Repeat("1", 40))
	c := Commit{
		Hash:      MustParseHash(strings.Repeat("a", 40)),
		Parents:   []Hash{parent},
		Author:    Signature{Name: "Ada", Email: "ada@example.com", When: when},
		Committer: Signature{Name: "Ada", Email: "ada@example.com", When: when.UTC()},
		Subject:   "subject",
		Message:   "subject\n\nbody\n",
	}
	if !c.Equal(c) {
		t.Fatal("commit must equal itself")
	}
	other := c
	other.Subject = "other"
	if c.Equal(other) {
		t.Fatal("subject must participate")
	}
	if !c.Author.Equal(Signature{Name: "Ada", Email: "ada@example.com", When: when}) {
		t.Fatal("signature")
	}
	// Same instant, different zone, is equal.
	if !c.Author.Equal(c.Committer) {
		t.Fatal("signatures at the same instant must be equal")
	}
}

func TestSubjectOf(t *testing.T) {
	tests := []struct{ in, want string }{
		{"subject", "subject"},
		{"subject\n\nbody", "subject"},
		{"subject\r\nbody", "subject"},
		{"", ""},
		{"one line\n", "one line"},
	}
	for _, tt := range tests {
		if got := SubjectOf(tt.in); got != tt.want {
			t.Errorf("SubjectOf(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseFileStatus(t *testing.T) {
	tests := []struct {
		in      string
		want    FileStatus
		wantErr bool
	}{
		{"A", StatusAdded, false},
		{"M", StatusModified, false},
		{"D", StatusDeleted, false},
		{"R", StatusRenamed, false},
		{"R100", StatusRenamed, false},
		{"C75", StatusCopied, false},
		{"T", StatusTypeChanged, false},
		{"U", StatusUnmerged, false},
		{"?", StatusUntracked, false},
		{"!", StatusIgnored, false},
		{"", StatusUnknown, true},
		{"X", StatusUnknown, true},
		{"a", StatusUnknown, true},
	}
	for _, tt := range tests {
		got, err := ParseFileStatus(TrimStatusToken(tt.in))
		if tt.wantErr {
			if !errors.Is(err, ErrUnknownStatus) {
				t.Errorf("ParseFileStatus(%q) err = %v", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseFileStatus(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
		if got.String() == "unknown" || got.Letter() == "" {
			t.Errorf("status %v has empty presentation", got)
		}
	}
}

func TestParseNameStatusLine(t *testing.T) {
	tests := []struct {
		in   string
		want FileChange
		bad  bool
	}{
		{"M\tsrc/a.go", FileChange{Status: StatusModified, Path: "src/a.go"}, false},
		{"A\tnew.go", FileChange{Status: StatusAdded, Path: "new.go"}, false},
		{"D\told.go", FileChange{Status: StatusDeleted, Path: "old.go"}, false},
		{"R100\told.go\tnew.go", FileChange{Status: StatusRenamed, Path: "new.go", OrigPath: "old.go"}, false},
		{"C50\ta.go\tb.go", FileChange{Status: StatusCopied, Path: "b.go", OrigPath: "a.go"}, false},
		{"T\tfile", FileChange{Status: StatusTypeChanged, Path: "file"}, false},
		{"R100\tonly-one", FileChange{}, true},
		{"nope", FileChange{}, true},
		{"X\tfile", FileChange{}, true},
	}
	for _, tt := range tests {
		got, err := ParseNameStatusLine(tt.in)
		if tt.bad {
			if err == nil {
				t.Errorf("ParseNameStatusLine(%q) err = nil", tt.in)
			}
			continue
		}
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("ParseNameStatusLine(%q) = %+v, %v; want %+v", tt.in, got, err, tt.want)
		}
	}
	renamed := FileChange{Status: StatusRenamed, Path: "b", OrigPath: "a"}
	if renamed.String() != "R a -> b" {
		t.Fatalf("string %q", renamed.String())
	}
}

func TestConflictSides(t *testing.T) {
	c := Conflict{Path: "f.txt", Ours: ConflictModified, Theirs: ConflictDeleted}
	if c.Status(SideOurs) != ConflictModified || c.Status(SideTheirs) != ConflictDeleted {
		t.Fatalf("direct sides: ours=%v theirs=%v", c.Status(SideOurs), c.Status(SideTheirs))
	}
	c.Reversed = true
	if c.Status(SideOurs) != ConflictDeleted || c.Status(SideTheirs) != ConflictModified {
		t.Fatalf("reversed sides: ours=%v theirs=%v", c.Status(SideOurs), c.Status(SideTheirs))
	}
	if SideOurs.String() != "ours" || SideTheirs.String() != "theirs" {
		t.Fatal("side names")
	}
	if ConflictAdded.String() != "added" || ConflictDeleted.String() != "deleted" {
		t.Fatal("stage names")
	}
}

func TestParseStashRef(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"stash@{0}", 0, false},
		{"stash@{1}", 1, false},
		{"stash@{12}", 12, false},
		{"stash@{01}", 0, true},
		{"stash@{}", 0, true},
		{"stash@{n}", 0, true},
		{"stash", 0, true},
		{"stash@{1", 0, true},
		{"refs/stash", 0, true},
		{"STASH@{0}", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseStashRef(tt.in)
		if tt.wantErr {
			if !errors.Is(err, ErrInvalidStashRef) {
				t.Errorf("ParseStashRef(%q) err = %v", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseStashRef(%q) = %d, %v; want %d", tt.in, got, err, tt.want)
		}
		formatted, ferr := FormatStashRef(got)
		if ferr != nil || formatted != tt.in {
			t.Errorf("FormatStashRef(%d) = %q, %v", got, formatted, ferr)
		}
	}
	if _, err := FormatStashRef(-1); !errors.Is(err, ErrInvalidStashRef) {
		t.Fatalf("negative index err = %v", err)
	}
}

func TestStashAndWorktree(t *testing.T) {
	when := time.Unix(1_700_000_000, 0).UTC()
	ref, err := FormatStashRef(0)
	if err != nil {
		t.Fatal(err)
	}
	s := Stash{
		Index:   0,
		Ref:     ref,
		Hash:    MustParseHash(strings.Repeat("a", 40)),
		Branch:  "WIP on master",
		Message: "WIP on master: abcdef0 subject",
		When:    when,
	}
	if !s.Equal(s) || s.Ref != "stash@{0}" {
		t.Fatalf("stash %+v", s)
	}

	branch, _ := LocalBranch("feature")
	w := Worktree{
		Path:   "/repo",
		HEAD:   s.Hash,
		Branch: branch,
		Main:   true,
	}
	if !w.OnBranch() || !w.Equal(w) {
		t.Fatal("worktree on branch")
	}
	detached := w
	detached.Branch = Ref{}
	detached.Detached = true
	if detached.OnBranch() || w.Equal(detached) {
		t.Fatal("detached worktree")
	}
	bare := Worktree{Path: "/repo.git", Bare: true}
	if bare.OnBranch() {
		t.Fatal("bare worktree")
	}

	sub := Submodule{Path: "vendor/lib", URL: "https://example.com/lib.git"}
	if !sub.Equal(sub) || sub.Equal(Submodule{Path: "vendor/lib"}) {
		t.Fatal("submodule")
	}
}

func TestRepoState(t *testing.T) {
	names := []struct {
		state RepoState
		name  string
	}{
		{StateNormal, "NORMAL"},
		{StateMerging, "MERGING"},
		{StateRebasing, "REBASING"},
		{StateGrafting, "GRAFTING"},
		{StateReverting, "REVERTING"},
		{StateDetached, "DETACHED"},
	}
	for _, tt := range names {
		if tt.state.String() != tt.name {
			t.Errorf("String() = %q, want %q", tt.state, tt.name)
		}
		got, err := ParseRepoState(tt.name)
		if err != nil || got != tt.state {
			t.Errorf("ParseRepoState(%q) = %v, %v", tt.name, got, err)
		}
	}
	if _, err := ParseRepoState("normal"); err == nil {
		t.Fatal("lowercase must not parse")
	}
	if _, err := ParseRepoState("CHERRY_PICKING"); err == nil {
		t.Fatal("cherry-picking is GRAFTING, not its own state")
	}
	if RepoState(99).String() == "" {
		t.Fatal("unknown state string")
	}
}

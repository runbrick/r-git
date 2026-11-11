package model

import (
	"errors"
	"testing"
)

func TestStripRefsPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"refs/heads/master", "master"},
		{"refs/heads/feature/sub", "feature/sub"},
		{"refs/remotes/origin/master", "origin/master"},
		{"refs/remotes/my/remote/master", "my/remote/master"},
		{"refs/tags/v1.0", "v1.0"},
		{"refs/tags/release/v1", "release/v1"},
		{"HEAD", "HEAD"},
		{"master", "master"},
		{"refs/for/master", "refs/for/master"},
		{"refs/heads", "refs/heads"},
		{"", ""},
		// Only a full prefix is stripped, and only one of them.
		{"refs/heads/refs/heads/x", "refs/heads/x"},
	}
	for _, tt := range tests {
		if got := StripRefsPrefix(tt.in); got != tt.want {
			t.Errorf("StripRefsPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAddRefsHeadsPrefix(t *testing.T) {
	tests := []struct{ in, want string }{
		{"master", "refs/heads/master"},
		{"refs/heads/master", "refs/heads/master"},
		{"feature/sub", "refs/heads/feature/sub"},
		{"", "refs/heads/"},
	}
	for _, tt := range tests {
		if got := AddRefsHeadsPrefix(tt.in); got != tt.want {
			t.Errorf("AddRefsHeadsPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		in      string
		kind    Kind
		name    string
		full    string
		wantErr bool
	}{
		{"refs/heads/master", KindLocalBranch, "master", "refs/heads/master", false},
		{"refs/heads/feature/sub", KindLocalBranch, "feature/sub", "refs/heads/feature/sub", false},
		{"refs/remotes/origin/master", KindRemoteBranch, "origin/master", "refs/remotes/origin/master", false},
		{"refs/remotes/my/remote/topic", KindRemoteBranch, "my/remote/topic", "refs/remotes/my/remote/topic", false},
		{"refs/tags/v1.0", KindTag, "v1.0", "refs/tags/v1.0", false},
		{"refs/tags/release/1.0", KindTag, "release/1.0", "refs/tags/release/1.0", false},
		{"HEAD", KindHead, "HEAD", "HEAD", false},
		{"master", KindUnknown, "", "", true},
		{"refs/for/master", KindUnknown, "", "", true},
		{"refs/heads/", KindUnknown, "", "", true},
		{"refs/remotes/", KindUnknown, "", "", true},
		{"refs/tags/", KindUnknown, "", "", true},
		{"", KindUnknown, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			ref, err := ParseRef(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidRef) {
					t.Fatalf("err = %v, want ErrInvalidRef", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ref.Kind() != tt.kind || ref.Name() != tt.name || ref.FullName() != tt.full || ref.String() != tt.full {
				t.Fatalf("got kind=%s name=%q full=%q, want kind=%s name=%q full=%q",
					ref.Kind(), ref.Name(), ref.FullName(), tt.kind, tt.name, tt.full)
			}
		})
	}
}

func TestLocalBranchConstructors(t *testing.T) {
	b, err := LocalBranch("refs/heads/feature/sub")
	if err != nil {
		t.Fatal(err)
	}
	short, err := LocalBranch("feature/sub")
	if err != nil {
		t.Fatal(err)
	}
	if !b.Equal(short) || b.FullName() != "refs/heads/feature/sub" {
		t.Fatalf("local branch mismatch %+v %+v", b, short)
	}
	if _, err := LocalBranch("refs/remotes/origin/master"); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("remote passed as local: %v", err)
	}
	if _, err := LocalBranch("refs/tags/v1"); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("tag passed as local: %v", err)
	}
	if _, err := LocalBranch("refs/heads/"); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("empty local: %v", err)
	}

	tag, err := TagRef("refs/tags/v1.0")
	if err != nil || tag.Name() != "v1.0" || tag.FullName() != "refs/tags/v1.0" {
		t.Fatalf("tag = %+v err %v", tag, err)
	}
	remote, err := RemoteBranchRef("origin/feature/sub")
	if err != nil || remote.Name() != "origin/feature/sub" || remote.Kind() != KindRemoteBranch {
		t.Fatalf("remote ref = %+v err %v", remote, err)
	}
}

func TestRefEqualityAndSort(t *testing.T) {
	prev := CaseSensitiveNames
	t.Cleanup(func() { CaseSensitiveNames = prev })

	CaseSensitiveNames = false
	a, _ := LocalBranch("Feature")
	b, _ := LocalBranch("feature")
	if !a.Equal(b) {
		t.Fatal("case-insensitive names must be equal")
	}
	CaseSensitiveNames = true
	if a.Equal(b) {
		t.Fatal("case-sensitive names must differ")
	}

	// Different kinds with the same short name are not equal.
	local, _ := LocalBranch("v1")
	tag, _ := TagRef("v1")
	if local.Equal(tag) {
		t.Fatal("local branch and tag must not be equal")
	}

	// Same kind sorts by short name, so feature2 < feature10 (natural order).
	f2, _ := LocalBranch("feature2")
	f10, _ := LocalBranch("feature10")
	if CompareRef(f2, f10) >= 0 {
		t.Fatalf("feature2 should sort before feature10, got %d", CompareRef(f2, f10))
	}
	// Different kinds sort by full name: refs/heads < refs/remotes < refs/tags.
	remote, _ := RemoteBranchRef("v1")
	if CompareRef(local, remote) >= 0 || CompareRef(remote, tag) >= 0 {
		t.Fatal("full-name kind order broken")
	}
	var zero Ref
	if CompareRef(zero, local) >= 0 || CompareRef(local, zero) <= 0 {
		t.Fatal("zero ref should sort first")
	}
}

func TestParseRemoteBranch(t *testing.T) {
	origin := Remote{Name: "origin", URLs: []string{"https://example.com/repo.git"}}
	slashRemote := Remote{Name: "my/remote", URLs: []string{"http://my.remote.git"}}
	remotes := []Remote{origin, slashRemote}

	tests := []struct {
		name    string
		in      string
		remotes []Remote
		remote  string
		branch  string
		local   string
		svn     bool
	}{
		{"ordinary", "refs/remotes/origin/master", remotes, "origin", "master", "origin/master", false},
		{"short form", "origin/master", remotes, "origin", "master", "origin/master", false},
		{"branch with slash", "refs/remotes/origin/feature/sub", remotes, "origin", "feature/sub", "origin/feature/sub", false},
		// Remote name itself contains a slash. The split must prefer the
		// configured remote over the first slash.
		{"remote with slash", "refs/remotes/my/remote/master", remotes, "my/remote", "master", "my/remote/master", false},
		{"remote with slash and branch slash", "my/remote/feature/sub", remotes, "my/remote", "feature/sub", "my/remote/feature/sub", false},
		// No configured remote matches: fall back to the first slash and
		// synthesize a remote. "origin/feature" is not the remote "origin"
		// when that remote is absent from the list.
		{"unknown remote", "refs/remotes/fork/topic", remotes, "fork", "topic", "fork/topic", false},
		{"ambiguous prefers configured", "refs/remotes/origin/feature/branch", []Remote{{Name: "origin/feature"}}, "origin/feature", "branch", "origin/feature/branch", false},
		{"no slash is svn", "refs/remotes/git-svn", remotes, ".", "refs/remotes/git-svn", "refs/remotes/git-svn", true},
		{"short no slash is svn", "trunk", nil, ".", "trunk", "trunk", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRemoteBranch(tt.in, tt.remotes)
			if got.Remote.Name != tt.remote {
				t.Errorf("remote = %q, want %q", got.Remote.Name, tt.remote)
			}
			if got.IsSVN() != tt.svn {
				t.Errorf("svn = %v, want %v", got.IsSVN(), tt.svn)
			}
			if tt.svn {
				if got.FullName() != tt.in || got.NameForLocalOperations() != tt.in || got.NameForRemoteOperations != tt.in {
					t.Errorf("svn names = full %q local %q remote-op %q, want %q",
						got.FullName(), got.NameForLocalOperations(), got.NameForRemoteOperations, tt.in)
				}
				if got.Remote.Name != "." {
					t.Errorf("svn remote = %q, want dot", got.Remote.Name)
				}
				return
			}
			if got.NameForRemoteOperations != tt.branch {
				t.Errorf("branch = %q, want %q", got.NameForRemoteOperations, tt.branch)
			}
			if got.NameForLocalOperations() != tt.local {
				t.Errorf("local name = %q, want %q", got.NameForLocalOperations(), tt.local)
			}
			if got.FullName() != RefsRemotesPrefix+tt.local {
				t.Errorf("full = %q", got.FullName())
			}
		})
	}
}

func TestRemoteBranchEqual(t *testing.T) {
	origin := Remote{Name: "origin", URLs: []string{"a"}}
	originOtherURL := Remote{Name: "origin", URLs: []string{"b"}}
	a := ParseRemoteBranch("refs/remotes/origin/master", []Remote{origin})
	b := ParseRemoteBranch("origin/master", []Remote{originOtherURL})
	if !a.Equal(b) {
		t.Fatal("same remote name and branch must be equal even when URLs differ")
	}
	other := ParseRemoteBranch("refs/remotes/origin/topic", []Remote{origin})
	if a.Equal(other) {
		t.Fatal("different branches compared equal")
	}
	if CompareRemoteBranch(a, other) >= 0 {
		t.Fatal("master should sort before topic")
	}
}

func TestRemoteEquality(t *testing.T) {
	a := Remote{Name: "origin", URLs: []string{"https://a"}}
	b := Remote{Name: "origin", URLs: []string{"https://b"}, PushURLs: []string{"https://push"}}
	if !a.Equal(b) {
		t.Fatal("remotes compare by name only")
	}
	if a.FirstURL() != "https://a" || b.FirstURL() != "https://b" {
		t.Fatal("first url")
	}
	empty := Remote{Name: "none"}
	if empty.FirstURL() != "" {
		t.Fatal("missing url")
	}
	if CompareRemote(Remote{Name: "a"}, Remote{Name: "b"}) >= 0 {
		t.Fatal("remote name order")
	}
	dot := DotRemote()
	if dot.Name != "." || dot.FirstURL() != "." {
		t.Fatalf("dot remote = %+v", dot)
	}
}

func TestTrackInfo(t *testing.T) {
	local, _ := LocalBranch("master")
	remote := ParseRemoteBranch("origin/master", []Remote{{Name: "origin"}})
	info := TrackInfo{Local: local, Remote: remote, Merge: true}
	if info.String() != "master->origin/master" {
		t.Fatalf("string %q", info.String())
	}
	same := TrackInfo{Local: local, Remote: remote, Merge: true}
	if !info.Equal(same) {
		t.Fatal("equal track info")
	}
	rebase := info
	rebase.Merge = false
	if info.Equal(rebase) {
		t.Fatal("merge flag must participate in equality")
	}
}

func TestSpecialRef(t *testing.T) {
	b := SpecialRefRemoteBranch("refs/for/master", Remote{Name: "origin"})
	if !b.IsSVN() && b.FullName() != "refs/for/master" {
		t.Fatal("special ref")
	}
	if b.FullName() != "refs/for/master" || b.NameForLocalOperations() != "refs/for/master" || b.NameForRemoteOperations != "refs/for/master" {
		t.Fatalf("special names full=%q local=%q remote=%q", b.FullName(), b.NameForLocalOperations(), b.NameForRemoteOperations)
	}
}

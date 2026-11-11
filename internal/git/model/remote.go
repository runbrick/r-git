package model

import "strings"

// Remote is one remote of a repository, as GitConfig presents it after URL
// rewriting (GitRemote).
//
// URLs are the fetch URLs (remote.<name>.url). PushURLs are the push URLs.
// GitConfig copies the fetch URL into PushURLs when the config has no
// pushURL, so a fully-parsed remote usually has both. This package does
// not perform that copy: it stores exactly the lists it is given, because
// the rewrite belongs to config parsing.
//
// FetchRefSpecs and PushRefSpecs are the raw refspecs. PushRefSpecs is
// empty when the config defines none, even if URLs were defaulted.
//
// Remotes compare and hash by name only. Two remotes with the same name
// and different URLs are equal, matching GitRemote.equals.
type Remote struct {
	Name          string
	URLs          []string
	PushURLs      []string
	FetchRefSpecs []string
	PushRefSpecs  []string
}

// OriginName is the conventional remote name (GitRemote.ORIGIN).
const OriginName = "origin"

// DotRemote is the synthetic remote git-svn uses for a branch config
// whose remote is "." (GitRemote.DOT).
func DotRemote() Remote {
	return Remote{Name: ".", URLs: []string{"."}}
}

// FirstURL returns the first fetch URL, or "" when the remote has none.
// Only the first URL is fetched; the others are ignored by Git.
func (r Remote) FirstURL() string {
	if len(r.URLs) == 0 {
		return ""
	}
	return r.URLs[0]
}

// Equal reports whether the remotes have the same name.
func (r Remote) Equal(other Remote) bool {
	return r.Name == other.Name
}

// CompareRemote orders remotes by name, byte-wise, matching
// String.compareTo (not the natural comparator used for ref names).
func CompareRemote(a, b Remote) int {
	return strings.Compare(a.Name, b.Name)
}

// String formats the remote the way GitRemote.toString does, with Go
// slice formatting for the URL and refspec lists.
func (r Remote) String() string {
	return "GitRemote(name='" + r.Name + "', urls=" + formatList(r.URLs) +
		", pushUrls=" + formatList(r.PushURLs) +
		", fetchRefSpecs=" + formatList(r.FetchRefSpecs) +
		", pushRefSpecs=" + formatList(r.PushRefSpecs) + ")"
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(item)
	}
	b.WriteByte(']')
	return b.String()
}

// TrackInfo is the upstream relationship of a local branch
// (GitBranchTrackInfo).
//
// Merge is true when branch.<name>.merge is set (the usual case) and false
// when the configured relationship is a rebase
// (branch.<name>.rebase). The Java field is named myMerge with the same
// meaning: true means merge, false means rebase.
type TrackInfo struct {
	Local  Ref
	Remote RemoteBranch
	Merge  bool
}

// Equal matches GitBranchTrackInfo.equals: local branch, remote branch,
// and the merge flag.
func (t TrackInfo) Equal(other TrackInfo) bool {
	return t.Local.Equal(other.Local) && t.Remote.Equal(other.Remote) && t.Merge == other.Merge
}

// String returns "local->remote" using short names, matching
// GitBranchTrackInfo.toString.
func (t TrackInfo) String() string {
	return t.Local.Name() + "->" + t.Remote.NameForLocalOperations()
}

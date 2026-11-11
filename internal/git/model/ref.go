package model

import (
	"errors"
	"fmt"
	"strings"
)

// Ref name prefixes. These match GitBranch.REFS_HEADS_PREFIX,
// GitBranch.REFS_REMOTES_PREFIX, and GitTag.REFS_TAGS_PREFIX.
const (
	RefsHeadsPrefix   = "refs/heads/"
	RefsRemotesPrefix = "refs/remotes/"
	RefsTagsPrefix    = "refs/tags/"
)

// HEAD is the symbolic name of the current revision.
const HEAD = "HEAD"

// ErrInvalidRef is returned when a string cannot be interpreted as a ref.
var ErrInvalidRef = errors.New("invalid git ref")

// Kind classifies a ref the way GitRefType.of does for a full name.
type Kind int

const (
	KindUnknown Kind = iota
	KindLocalBranch
	KindRemoteBranch
	KindTag
	KindHead
)

// String returns the kind name used by GitBranchType/GitTagType
// (LOCAL, REMOTE, TAG, HEAD). Unknown is "UNKNOWN".
func (k Kind) String() string {
	switch k {
	case KindLocalBranch:
		return "LOCAL"
	case KindRemoteBranch:
		return "REMOTE"
	case KindTag:
		return "TAG"
	case KindHead:
		return "HEAD"
	default:
		return "UNKNOWN"
	}
}

// Ref is a named Git reference: a local branch, a remote-tracking branch,
// a tag, or HEAD.
//
// Name is the short name (GitReference.name): the part after refs/heads/,
// refs/remotes/, or refs/tags/. FullName is the full ref
// (GitReference.fullName). For HEAD both are "HEAD".
//
// Two refs are equal when their short names compare equal with the active
// case rule (see CaseSensitiveNames) and they are the same Go type. A local
// branch and a tag that share a short name are not equal. This matches
// GitReference.equals, which compares the short name and requires the same
// concrete class.
type Ref struct {
	kind Kind
	name string // short name
}

// Kind returns the ref kind.
func (r Ref) Kind() Kind { return r.kind }

// Name returns the short name ("feature", "origin/master", "v1.0", "HEAD").
func (r Ref) Name() string { return r.name }

// FullName returns the full ref name.
func (r Ref) FullName() string {
	switch r.kind {
	case KindLocalBranch:
		return RefsHeadsPrefix + r.name
	case KindRemoteBranch:
		return RefsRemotesPrefix + r.name
	case KindTag:
		return RefsTagsPrefix + r.name
	case KindHead:
		return HEAD
	default:
		return r.name
	}
}

// String returns the full name, matching GitReference.toString.
func (r Ref) String() string { return r.FullName() }

// IsZero reports whether r is the zero Ref.
func (r Ref) IsZero() bool { return r.kind == KindUnknown && r.name == "" }

// LocalBranch constructs a local branch from a short or full name.
// "refs/heads/feature" and "feature" both produce the short name "feature".
// A full remote-tracking or tag name is rejected, and so is a name that is
// empty once the heads prefix is removed.
func LocalBranch(name string) (Ref, error) {
	if strings.HasPrefix(name, RefsRemotesPrefix) || strings.HasPrefix(name, RefsTagsPrefix) {
		return Ref{}, fmt.Errorf("%w: local branch %q", ErrInvalidRef, name)
	}
	short := name
	if strings.HasPrefix(name, RefsHeadsPrefix) {
		short = name[len(RefsHeadsPrefix):]
	}
	if short == "" {
		return Ref{}, fmt.Errorf("%w: local branch %q", ErrInvalidRef, name)
	}
	return Ref{kind: KindLocalBranch, name: short}, nil
}

// RemoteBranchRef constructs a remote-tracking branch from a short or full
// name without splitting the remote. "refs/remotes/origin/master" and
// "origin/master" both produce the short name "origin/master".
// Use ParseRemoteBranch when the remote and the branch must be separated
// with a configured remote list.
func RemoteBranchRef(name string) (Ref, error) {
	short := name
	if strings.HasPrefix(name, RefsRemotesPrefix) {
		short = name[len(RefsRemotesPrefix):]
	} else if strings.HasPrefix(name, RefsHeadsPrefix) || strings.HasPrefix(name, RefsTagsPrefix) {
		return Ref{}, fmt.Errorf("%w: remote branch %q", ErrInvalidRef, name)
	}
	if short == "" {
		return Ref{}, fmt.Errorf("%w: remote branch %q", ErrInvalidRef, name)
	}
	return Ref{kind: KindRemoteBranch, name: short}, nil
}

// TagRef constructs a tag from a short or full name.
func TagRef(name string) (Ref, error) {
	short := name
	if strings.HasPrefix(name, RefsTagsPrefix) {
		short = name[len(RefsTagsPrefix):]
	} else if strings.HasPrefix(name, RefsHeadsPrefix) || strings.HasPrefix(name, RefsRemotesPrefix) {
		return Ref{}, fmt.Errorf("%w: tag %q", ErrInvalidRef, name)
	}
	if short == "" {
		return Ref{}, fmt.Errorf("%w: tag %q", ErrInvalidRef, name)
	}
	return Ref{kind: KindTag, name: short}, nil
}

// HeadRef returns the HEAD ref.
func HeadRef() Ref { return Ref{kind: KindHead, name: HEAD} }

// ParseRef classifies a full ref name.
//
//	refs/heads/feature       -> local branch "feature"
//	refs/remotes/origin/main -> remote branch "origin/main"
//	refs/tags/v1.0           -> tag "v1.0"
//	HEAD                     -> HEAD
//
// Anything else, including a short branch name, is rejected. Short names
// are ambiguous between branches and tags; use LocalBranch, RemoteBranchRef,
// or TagRef when the caller already knows the kind.
func ParseRef(full string) (Ref, error) {
	switch {
	case full == HEAD:
		return HeadRef(), nil
	case strings.HasPrefix(full, RefsHeadsPrefix):
		return LocalBranch(full)
	case strings.HasPrefix(full, RefsRemotesPrefix):
		return RemoteBranchRef(full)
	case strings.HasPrefix(full, RefsTagsPrefix):
		return TagRef(full)
	default:
		return Ref{}, fmt.Errorf("%w: %q", ErrInvalidRef, full)
	}
}

// StripRefsPrefix removes a leading refs/heads/, refs/remotes/, or refs/tags/
// prefix. Other strings, including "HEAD" and "refs/for/master", are returned
// unchanged. This matches com.intellij.vcs.git.ref.GitRefUtil.stripRefsPrefix.
func StripRefsPrefix(refName string) string {
	return stripKnownPrefix(refName)
}

func stripKnownPrefix(refName string) string {
	for _, prefix := range []string{RefsHeadsPrefix, RefsRemotesPrefix, RefsTagsPrefix} {
		if strings.HasPrefix(refName, prefix) {
			return refName[len(prefix):]
		}
	}
	return refName
}

// AddRefsHeadsPrefix returns refs/heads/ + name when name does not already
// start with that prefix. An empty name is returned unchanged, matching
// GitRefUtil.addRefsHeadsPrefixIfNeeded for a non-null input that is empty
// (the Java method only skips a null).
func AddRefsHeadsPrefix(branchName string) string {
	if strings.HasPrefix(branchName, RefsHeadsPrefix) {
		return branchName
	}
	return RefsHeadsPrefix + branchName
}

// Equal reports whether r and other are the same ref under the active
// case rule. Kinds must match; short names are compared with
// CaseSensitiveNames.
func (r Ref) Equal(other Ref) bool {
	if r.kind != other.kind {
		return false
	}
	return namesEqual(r.name, other.name)
}

// Compare orders refs the way GitReference.compareTo does.
//
// Refs of the same concrete kind are ordered by short name with
// NaturalCompare, which is what the specialized compareTo overrides do
// (they skip building the full name because the prefix is shared).
// Refs of different kinds are ordered by full name, so "refs/heads/..."
// sorts before "refs/remotes/..." which sorts before "refs/tags/...".
// A zero Ref sorts before any non-zero ref; Java treats a null argument
// as smaller (compareTo returns 1 when other is null).
func CompareRef(a, b Ref) int {
	if a.IsZero() && b.IsZero() {
		return 0
	}
	if a.IsZero() {
		return -1
	}
	if b.IsZero() {
		return 1
	}
	if a.kind == b.kind {
		return NaturalCompare(a.name, b.name)
	}
	return NaturalCompare(a.FullName(), b.FullName())
}

// namesEqual compares ref short names. When CaseSensitiveNames is false
// (the Windows/macOS default in git4idea) the comparison is case-insensitive.
func namesEqual(a, b string) bool {
	if CaseSensitiveNames {
		return a == b
	}
	return strings.EqualFold(a, b)
}

// CaseSensitiveNames selects the branch-name hashing strategy.
//
// git4idea uses a case-insensitive strategy when the filesystem is
// case-insensitive (CaseSensitivityInfoHolder) and a case-sensitive one
// otherwise. The zero value is false, matching the common Windows checkout
// this port is developed on. Callers on a case-sensitive repository set it
// to true. It is a package-level switch because Java stored the choice in
// a process-wide holder; tests that change it must restore it.
var CaseSensitiveNames = false

// RemoteBranch is a remote-tracking branch split into its remote and the
// name the branch has on that remote.
//
// NameForLocalOperations is how the branch is named in this repository
// ("origin/master" or "my/remote/master"). NameForRemoteOperations is the
// name on the remote ("master"). FullName is "refs/remotes/" plus the
// local name for an ordinary branch. SVN-style branches (no slash, or an
// explicit svn remote) keep the original ref as all three names, matching
// GitSvnRemoteBranch.
type RemoteBranch struct {
	Remote                  Remote
	NameForRemoteOperations string
	svn                     bool
	// raw is the original ref for an SVN-style branch, whose full name is
	// not refs/remotes/ plus the short name.
	raw string
}

// NameForLocalOperations returns the name used by local operations
// (checkout, merge, rebase), for example "origin/master". For an
// SVN-style branch this is the original ref.
func (b RemoteBranch) NameForLocalOperations() string {
	if b.svn {
		return b.raw
	}
	return b.Remote.Name + "/" + b.NameForRemoteOperations
}

// FullName returns the full ref. Ordinary branches use refs/remotes/;
// SVN-style branches return the original ref unchanged.
func (b RemoteBranch) FullName() string {
	if b.svn {
		return b.raw
	}
	return RefsRemotesPrefix + b.NameForLocalOperations()
}

// Ref returns the remote branch as a Ref. SVN-style branches are returned
// as a remote ref whose short name is the original string, because they
// have no refs/remotes/ short form of their own.
func (b RemoteBranch) Ref() Ref {
	if b.svn {
		return Ref{kind: KindRemoteBranch, name: StripRefsPrefix(b.raw)}
	}
	return Ref{kind: KindRemoteBranch, name: b.NameForLocalOperations()}
}

// IsSVN reports whether this is a git-svn remote branch (GitSvnRemoteBranch).
func (b RemoteBranch) IsSVN() bool { return b.svn }

// Equal reports whether two remote branches are the same.
//
// GitStandardRemoteBranch.equals requires equal short names (via super),
// equal nameForRemoteOperations, and equal remotes (remotes compare by
// name only). SVN branches compare by their raw ref.
func (b RemoteBranch) Equal(other RemoteBranch) bool {
	if b.svn || other.svn {
		return b.svn == other.svn && b.raw == other.raw
	}
	return namesEqual(b.NameForLocalOperations(), other.NameForLocalOperations()) &&
		b.NameForRemoteOperations == other.NameForRemoteOperations &&
		b.Remote.Equal(other.Remote)
}

// ParseRemoteBranch splits a remote-tracking branch into remote and branch
// using the configured remotes.
//
// The algorithm is GitUtil.parseRemoteBranch:
//
//  1. Strip refs/heads/, refs/remotes/, or refs/tags/.
//  2. If the remainder contains no '/', it is a git-svn branch and the
//     remote is GitRemote.DOT (".").
//  3. Otherwise walk slash positions from the left and take the longest
//     prefix that equals a configured remote name. This is what makes
//     remote names containing '/' work: "my/remote/master" with a remote
//     named "my/remote" yields branch "master", not remote "my".
//  4. If no configured remote matches, assume the remote is the segment
//     before the first slash and synthesize a remote with that name and
//     no URLs. JetBrains prefers this because remote names with slashes
//     are less common than branch names with slashes.
func ParseRemoteBranch(fullBranchName string, remotes []Remote) RemoteBranch {
	stdName := StripRefsPrefix(fullBranchName)
	slash := strings.IndexByte(stdName, '/')
	if slash == -1 {
		return RemoteBranch{
			Remote:                  DotRemote(),
			NameForRemoteOperations: fullBranchName,
			svn:                     true,
			raw:                     fullBranchName,
		}
	}
	rest := stdName
	for {
		remoteName := rest[:slash]
		branchName := rest[slash+1:]
		if remote, ok := findRemote(remotes, remoteName); ok {
			return RemoteBranch{
				Remote:                  remote,
				NameForRemoteOperations: StripRefsPrefix(branchName),
			}
		}
		next := strings.IndexByte(rest[slash+1:], '/')
		if next < 0 {
			break
		}
		slash = slash + 1 + next
	}
	first := strings.IndexByte(stdName, '/')
	return RemoteBranch{
		Remote: Remote{
			Name: stdName[:first],
		},
		NameForRemoteOperations: StripRefsPrefix(stdName[first+1:]),
	}
}

func findRemote(remotes []Remote, name string) (Remote, bool) {
	for _, r := range remotes {
		if r.Name == name {
			return r, true
		}
	}
	return Remote{}, false
}

// SpecialRefRemoteBranch is the semi-fake remote branch used when pushing
// to a non-branch refspec such as "HEAD:refs/for/master"
// (GitSpecialRefRemoteBranch). All three names are the raw ref.
func SpecialRefRemoteBranch(ref string, remote Remote) RemoteBranch {
	return RemoteBranch{
		Remote:                  remote,
		NameForRemoteOperations: ref,
		svn:                     true,
		raw:                     ref,
	}
}

// CompareRemoteBranch orders two ordinary remote branches by their local
// name (GitStandardRemoteBranch.compareTo). SVN branches participate by
// their raw ref.
func CompareRemoteBranch(a, b RemoteBranch) int {
	return NaturalCompare(a.NameForLocalOperations(), b.NameForLocalOperations())
}

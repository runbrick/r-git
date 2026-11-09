// Package model holds the pure Git data types used by the rest of r-git.
//
// Nothing in this package reads the filesystem, runs git, or imports a UI
// toolkit. Callers that talk to a repository live in internal/git/repo and
// internal/git/ops.
//
// The shapes follow the JetBrains git4idea types (GitReference, GitRemote,
// GitCommit, Repository.State, StashInfo, and the file-status enums) but are
// plain Go values: composition, sentinel errors, and functions for the
// equality and sort rules Java expressed with equals/compareTo.
package model

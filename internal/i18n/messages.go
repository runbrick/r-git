// Package i18n contains original English and Simplified Chinese UI messages.
//
// Glossary: commit 提交; push 推送; pull 拉取; fetch 获取; branch 分支;
// tag 标签; remote 远程; merge 合并; rebase 变基; interactive rebase 交互式变基;
// cherry-pick 拣选; revert 还原; reset 重置; stash 储藏; stage 暂存;
// unstage 取消暂存; rollback 回滚; conflict 冲突; resolve 解决;
// amend 修改上次提交; unversioned files 未版本控制的文件; changes 变更;
// log 日志; author 作者; date 日期. Git output and user content are never translated.
package i18n

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// Messages is a complete, typed catalogue. Positional bundle literals ensure
// adding a message requires updating both languages at compile time.
type Messages struct {
	CheckoutRestoreError   string
	BranchFilter           string
	BareRepository         string
	InvalidSettings        string
	UnknownSettingsVersion string
	PushSuccess            string
	PushForced             string
	PushNew                string
	PushRejected           string
	PushDeleted            string
	PushUpToDate           string
	PushError              string
	StatusAdded            string
	StatusModified         string
	StatusDeleted          string
	StatusRenamed          string
	StatusCopied           string
	StatusUnmerged         string
	StatusUntracked        string
	StatusTypeChanged      string
	YoursRebasing          string
	TheirsUpstream         string
	RebaseDone             string
	RebaseNothing          string
	RebaseConflicts        string
	RebaseStopped          string

	RunningGit      string
	LoadingConflict string
	InvalidDiffPath string
	BinaryDiff      string
	NoCommitsRebase string
	HistoryChanged  string
	InvalidTodo     string
	OperationFailed string
	NotRepository   string
	CloseDialog     string
	Yours           string
	Theirs          string
	Base            string
	Reset           string
	Revert          string
	CherryPick      string

	Commit                                                                                      string
	CommitAndPush                                                                               string
	CommitAndPushEllipsis                                                                       string
	Push                                                                                        string
	Pull                                                                                        string
	Fetch                                                                                       string
	Branches                                                                                    string
	Conflicts                                                                                   string
	InteractiveRebase                                                                           string
	Diff                                                                                        string
	Settings                                                                                    string
	Refresh                                                                                     string
	OpenRepository                                                                              string
	OpenStash                                                                                   string
	RecentProjects                                                                              string
	RecentRepositories                                                                          string
	Appearance                                                                                  string
	Theme                                                                                       string
	OK                                                                                          string
	Open                                                                                        string
	RepositoryPath                                                                              string
	PathToAGitWorkingTree                                                                       string
	LoadingRepositoryEllipsis                                                                   string
	CommitFailed                                                                                string
	Committed                                                                                   string
	Changes                                                                                     string
	Staged                                                                                      string
	Unstaged                                                                                    string
	Discarded                                                                                   string
	RebaseInProgress                                                                            string
	ContinueRebase                                                                              string
	SkipCommit                                                                                  string
	AbortRebase                                                                                 string
	RebaseStoppedForEditAmendTheCommitThenContinueTheRebase                                     string
	Cancel                                                                                      string
	LocalChangesWouldBeOverwrittenByCheckout                                                    string
	StashAndCheckout                                                                            string
	NewBranch                                                                                   string
	BranchName                                                                                  string
	CreateBranch                                                                                string
	EnterABranchName                                                                            string
	NewName                                                                                     string
	Rename                                                                                      string
	Delete                                                                                      string
	ForceDelete                                                                                 string
	PushResults                                                                                 string
	Close                                                                                       string
	PullThenPush                                                                                string
	PushCommits                                                                                 string
	SelectABranchToPush                                                                         string
	NothingToPush                                                                               string
	Remote                                                                                      string
	TargetBranch                                                                                string
	NoOutgoingCommits                                                                           string
	ForceWithLease                                                                              string
	PushTags                                                                                    string
	PushProducedNoPerRefResults                                                                 string
	ABranchWasRejectedPullThenPushAgain                                                         string
	Branch                                                                                      string
	UpdateMethod                                                                                string
	Merge                                                                                       string
	Rebase                                                                                      string
	PullNow                                                                                     string
	EnterARemote                                                                                string
	Local                                                                                       string
	Tags                                                                                        string
	FetchRemote                                                                                 string
	PullChanges                                                                                 string
	PushBranches                                                                                string
	Stash                                                                                       string
	Remotes                                                                                     string
	BranchesTree                                                                                string
	BranchActions                                                                               string
	Checkout                                                                                    string
	NewBranchFrom                                                                               string
	MergeIntoCurrent                                                                            string
	RebaseCurrentOnto                                                                           string
	Compare                                                                                     string
	CheckoutAndRebase                                                                           string
	Favorite                                                                                    string
	Unfavorite                                                                                  string
	FindBranch                                                                                  string
	TypeToFilter                                                                                string
	LocalChangesNotFullyRestored                                                                string
	TheStashWasKeptOpenConflictsToResolveTheRemainingChangesOrOpenStashToInspectTheSavedChanges string
	FetchFailed                                                                                 string
	Fetched                                                                                     string
	Fetching                                                                                    string
	GitError                                                                                    string
	Pushed                                                                                      string
	RemotesSaved                                                                                string
	StashUpdated                                                                                string
	GitCredentials                                                                              string
	Username                                                                                    string
	Password                                                                                    string
	Stashes                                                                                     string
	NoStashes                                                                                   string
	CreateStash                                                                                 string
	Message                                                                                     string
	IncludeUntracked                                                                            string
	KeepIndex                                                                                   string
	CloseStash                                                                                  string
	SaveStash                                                                                   string
	Apply                                                                                       string
	Pop                                                                                         string
	Drop                                                                                        string
	DropStash                                                                                   string
	PopStash                                                                                    string
	PopNow                                                                                      string
	Name                                                                                        string
	URL                                                                                         string
	Remove                                                                                      string
	AddRemote                                                                                   string
	NewRemoteName                                                                               string
	NewRemoteURL                                                                                string
	Save                                                                                        string
	ANewRemoteNeedsANameAndAURL                                                                 string
	RemoteNameCannotBeEmpty                                                                     string
	NothingToCommitStageAChangeOrIncludeAFile                                                   string
	GPGCouldNotSignTheCommit                                                                    string
	UnmergedFilesMustBeResolvedBeforeCommitting                                                 string
	TheOperationStoppedOnAMergeConflict                                                         string
	ThatCommitIsAlreadyApplied                                                                  string
	LocalChangesWouldBeOverwritten                                                              string
	UntrackedFilesWouldBeOverwritten                                                            string
	TheBranchIsAlreadyCheckedOutInAnotherWorktree                                               string
	RefreshChanges                                                                              string
	CommitOptions                                                                               string
	SignOff                                                                                     string
	Author                                                                                      string
	NameEmail                                                                                   string
	NoChanges                                                                                   string
	Stage                                                                                       string
	Unstage                                                                                     string
	Rollback                                                                                    string
	ShowDiff                                                                                    string
	RollbackChanges                                                                             string
	Discard                                                                                     string
	Amend                                                                                       string
	LastCommit                                                                                  string
	AmendLastCommit                                                                             string
	CommitMessage                                                                               string
	DescribeTheChange                                                                           string
	CommitChanges                                                                               string
	EnterACommitMessage                                                                         string
	UnversionedFiles                                                                            string
	PreviousChange                                                                              string
	NextChange                                                                                  string
	Unified                                                                                     string
	Default                                                                                     string
	IgnoreWhitespace                                                                            string
	IgnoreTrailingWhitespace                                                                    string
	ComparisonPolicy                                                                            string
	CloseDiff                                                                                   string
	DiffContentF7NextShiftF7Previous                                                            string
	InvalidRegularExpression                                                                    string
	Filter                                                                                      string
	TextOrHash                                                                                  string
	Regex                                                                                       string
	MatchCase                                                                                   string
	User                                                                                        string
	FilterByAuthor                                                                              string
	UserFilter                                                                                  string
	AuthorContains                                                                              string
	Date                                                                                        string
	RefreshLog                                                                                  string
	ShowDetails                                                                                 string
	LogOptions                                                                                  string
	ShowHash                                                                                    string
	Subject                                                                                     string
	Hash                                                                                        string
	LoadingEllipsis                                                                             string
	ChangedFiles                                                                                string
	SelectACommit                                                                               string
	CloseConflicts                                                                              string
	SelectAConflictedFile                                                                       string
	YoursContent                                                                                string
	TheirsContent                                                                               string
	Result                                                                                      string
	MergeResult                                                                                 string
	Change                                                                                      string
	Conflict                                                                                    string
	Accepted                                                                                    string
	AcceptYours                                                                                 string
	AcceptTheirs                                                                                string
	ConflictChunks                                                                              string
	ApplyNonConflictingChanges                                                                  string
	ResultEditedManually                                                                        string
	ResetResultToChunkChoices                                                                   string
	MarkResolved                                                                                string
	Upstream                                                                                    string
	LoadCommits                                                                                 string
	StartRebase                                                                                 string
	Language                                                                                    string
	Dark                                                                                        string
	Light                                                                                       string
	AllBranches                                                                                 string
	AnyDate                                                                                     string
	Today                                                                                       string
	Last7Days                                                                                   string
	Last30Days                                                                                  string
	BeforeToday                                                                                 string
	StagedChanges                                                                               string
	Pick                                                                                        string
	Reword                                                                                      string
	Edit                                                                                        string
	Squash                                                                                      string
	Fixup                                                                                       string
	DropText                                                                                    string
	LogTitle                                                                                    func(branch string) string
	CheckoutTitle                                                                               func(name string) string
	StashCheckoutPrompt                                                                         func(name string) string
	FromRef                                                                                     func(name string) string
	RenameTitle                                                                                 func(name string) string
	DeleteTitle                                                                                 func(name string) string
	DeleteBranchPrompt                                                                          func(name string) string
	NotFullyMerged                                                                              func(branch, base string) string
	UnmergedCount                                                                               func(n int) string
	CredentialsTitle                                                                            func(url string) string
	CurrentBranch                                                                               func(name string) string
	CreatedBranch                                                                               func(name string) string
	RenamedBranch                                                                               func(name string) string
	DeletedBranch                                                                               func(name string) string
	CheckedOut                                                                                  func(name string) string
	PulledBranch                                                                                func(name string) string
	MergedBranch                                                                                func(name string) string
	RebasedBranch                                                                               func(name string) string
	CheckedOutRebased                                                                           func(name string) string
	MergeConflicts                                                                              func(name string) string
	RefsCount                                                                                   func(n int) string
	DropStashPrompt                                                                             func(ref string) string
	PopStashPrompt                                                                              func(ref string) string
	DiscardPrompt                                                                               func(path string) string
	IncludePath                                                                                 func(path string) string
	ChangePosition                                                                              func(current, total int) string
	AuthorName                                                                                  func(name string) string
	CommitterName                                                                               func(name string) string
	AuthoredOn                                                                                  func(date string) string
	CommittedOn                                                                                 func(date string) string
	ParentCommits                                                                               func(hashes string) string
	InBranches                                                                                  func(n int, names string) string
	ActionNumber                                                                                func(n int) string
	MessageNumber                                                                               func(n int) string
	MoveUp                                                                                      func(n int) string
	MoveDown                                                                                    func(n int) string
	AcceptYoursNumber                                                                           func(n int) string
	AcceptTheirsNumber                                                                          func(n int) string
	ChunkDescription                                                                            func(kind string, n, start, end int) string
	RelativeDate                                                                                func(when, now time.Time) string
	CommitsCount                                                                                func(int) string
	FilesCount                                                                                  func(int) string
	ShortDate                                                                                   func(time.Time) string
}

var En = Messages{
	"Local changes could not be fully restored; stash kept (%s): %v", // CheckoutRestoreError
	"Branch",                              // BranchFilter
	"bare repositories are not supported", // BareRepository
	"Invalid settings file",               // InvalidSettings
	"Unknown settings version",            // UnknownSettingsVersion
	"SUCCESS",                             // PushSuccess
	"FORCED_UPDATE",                       // PushForced
	"NEW_REF",                             // PushNew
	"REJECTED",                            // PushRejected
	"DELETED",                             // PushDeleted
	"UP_TO_DATE",                          // PushUpToDate
	"ERROR",                               // PushError
	"added",                               // StatusAdded
	"modified",                            // StatusModified
	"deleted",                             // StatusDeleted
	"renamed",                             // StatusRenamed
	"copied",                              // StatusCopied
	"unmerged",                            // StatusUnmerged
	"untracked",                           // StatusUntracked
	"type changed",                        // StatusTypeChanged
	"Yours (rebasing branch)",             // YoursRebasing
	"Theirs (upstream)",                   // TheirsUpstream
	"Rebase completed",                    // RebaseDone
	"Nothing to rebase",                   // RebaseNothing
	"Rebase stopped on conflicts",         // RebaseConflicts
	"Rebase stopped for edit",             // RebaseStopped

	"Running Git…",                                   // RunningGit
	"Loading or saving conflict…",                    // LoadingConflict
	"invalid diff path",                              // InvalidDiffPath
	"binary file: text diff is unavailable",          // BinaryDiff
	"no commits to rebase",                           // NoCommitsRebase
	"rebase history changed; load the commits again", // HistoryChanged
	"Invalid rebase plan. Check actions and messages; squash/fixup need a preceding commit, and only one reword is supported.", // InvalidTodo
	"Git operation failed", // OperationFailed
	"not a git repository", // NotRepository
	"Close dialog",         // CloseDialog
	"Yours",                // Yours
	"Theirs",               // Theirs
	"Base",                 // Base
	"Reset",                // Reset
	"Revert",               // Revert
	"Cherry-pick",          // CherryPick

	"Commit",                     // Commit
	"Commit and Push",            // CommitAndPush
	"Commit and Push...",         // CommitAndPushEllipsis
	"Push",                       // Push
	"Pull",                       // Pull
	"Fetch",                      // Fetch
	"Branches",                   // Branches
	"Conflicts",                  // Conflicts
	"Interactive rebase",         // InteractiveRebase
	"Diff",                       // Diff
	"Settings",                   // Settings
	"Refresh",                    // Refresh
	"Open repository",            // OpenRepository
	"Open stash",                 // OpenStash
	"Recent projects",            // RecentProjects
	"Recent repositories",        // RecentRepositories
	"Appearance",                 // Appearance
	"Theme",                      // Theme
	"OK",                         // OK
	"Open",                       // Open
	"Repository path",            // RepositoryPath
	"Path to a Git working tree", // PathToAGitWorkingTree
	"Loading repository...",      // LoadingRepositoryEllipsis
	"Commit failed",              // CommitFailed
	"Committed",                  // Committed
	"Changes",                    // Changes
	"Staged",                     // Staged
	"Unstaged",                   // Unstaged
	"Discarded",                  // Discarded
	"Rebase in progress",         // RebaseInProgress
	"Continue rebase",            // ContinueRebase
	"Skip commit",                // SkipCommit
	"Abort rebase",               // AbortRebase
	"Rebase stopped for edit; amend the commit, then continue the rebase.", // RebaseStoppedForEditAmendTheCommitThenContinueTheRebase
	"Cancel", // Cancel
	"Local changes would be overwritten by checkout.", // LocalChangesWouldBeOverwrittenByCheckout
	"Stash and checkout",                              // StashAndCheckout
	"New branch",                                      // NewBranch
	"Branch name",                                     // BranchName
	"Create branch",                                   // CreateBranch
	"Enter a branch name.",                            // EnterABranchName
	"New name",                                        // NewName
	"Rename",                                          // Rename
	"Delete",                                          // Delete
	"Force delete",                                    // ForceDelete
	"Push results",                                    // PushResults
	"Close",                                           // Close
	"Pull then push",                                  // PullThenPush
	"Push commits",                                    // PushCommits
	"Select a branch to push.",                        // SelectABranchToPush
	"Nothing to push.",                                // NothingToPush
	"Remote",                                          // Remote
	"Target branch",                                   // TargetBranch
	"No outgoing commits",                             // NoOutgoingCommits
	"Force with lease",                                // ForceWithLease
	"Push tags",                                       // PushTags
	"Push produced no per-ref results.",               // PushProducedNoPerRefResults
	"A branch was rejected. Pull, then push again?", // ABranchWasRejectedPullThenPushAgain
	"Branch",                           // Branch
	"Update method",                    // UpdateMethod
	"Merge",                            // Merge
	"Rebase",                           // Rebase
	"Pull now",                         // PullNow
	"Enter a remote.",                  // EnterARemote
	"Local",                            // Local
	"Tags",                             // Tags
	"Fetch remote",                     // FetchRemote
	"Pull changes",                     // PullChanges
	"Push branches",                    // PushBranches
	"Stash",                            // Stash
	"Remotes",                          // Remotes
	"Branches tree",                    // BranchesTree
	"Branch actions",                   // BranchActions
	"Checkout",                         // Checkout
	"New branch from",                  // NewBranchFrom
	"Merge into current",               // MergeIntoCurrent
	"Rebase current onto",              // RebaseCurrentOnto
	"Compare",                          // Compare
	"Checkout and rebase",              // CheckoutAndRebase
	"Favorite",                         // Favorite
	"Unfavorite",                       // Unfavorite
	"Find branch",                      // FindBranch
	"Type to filter",                   // TypeToFilter
	"Local changes not fully restored", // LocalChangesNotFullyRestored
	"The stash was kept. Open Conflicts to resolve the remaining changes, or Open stash to inspect the saved changes.", // TheStashWasKeptOpenConflictsToResolveTheRemainingChangesOrOpenStashToInspectTheSavedChanges
	"Fetch failed",                         // FetchFailed
	"Fetched",                              // Fetched
	"Fetching",                             // Fetching
	"Git error",                            // GitError
	"Pushed",                               // Pushed
	"Remotes saved",                        // RemotesSaved
	"Stash updated",                        // StashUpdated
	"Git credentials",                      // GitCredentials
	"Username",                             // Username
	"Password",                             // Password
	"Stashes",                              // Stashes
	"No stashes.",                          // NoStashes
	"Create stash",                         // CreateStash
	"Message",                              // Message
	"Include untracked",                    // IncludeUntracked
	"Keep index",                           // KeepIndex
	"Close stash",                          // CloseStash
	"Save stash",                           // SaveStash
	"Apply",                                // Apply
	"Pop",                                  // Pop
	"Drop",                                 // Drop
	"Drop stash",                           // DropStash
	"Pop stash",                            // PopStash
	"Pop now",                              // PopNow
	"Name",                                 // Name
	"URL",                                  // URL
	"Remove",                               // Remove
	"Add remote",                           // AddRemote
	"New remote name",                      // NewRemoteName
	"New remote URL",                       // NewRemoteURL
	"Save",                                 // Save
	"A new remote needs a name and a URL.", // ANewRemoteNeedsANameAndAURL
	"Remote name cannot be empty.",         // RemoteNameCannotBeEmpty
	"Nothing to commit. Stage a change or include a file.",   // NothingToCommitStageAChangeOrIncludeAFile
	"GPG could not sign the commit.",                         // GPGCouldNotSignTheCommit
	"Unmerged files must be resolved before committing.",     // UnmergedFilesMustBeResolvedBeforeCommitting
	"The operation stopped on a merge conflict.",             // TheOperationStoppedOnAMergeConflict
	"That commit is already applied.",                        // ThatCommitIsAlreadyApplied
	"Local changes would be overwritten.",                    // LocalChangesWouldBeOverwritten
	"Untracked files would be overwritten.",                  // UntrackedFilesWouldBeOverwritten
	"The branch is already checked out in another worktree.", // TheBranchIsAlreadyCheckedOutInAnotherWorktree
	"Refresh changes",            // RefreshChanges
	"Commit options",             // CommitOptions
	"Sign-off",                   // SignOff
	"Author",                     // Author
	"Name <email>",               // NameEmail
	"No changes",                 // NoChanges
	"Stage",                      // Stage
	"Unstage",                    // Unstage
	"Rollback",                   // Rollback
	"Show diff",                  // ShowDiff
	"Rollback changes",           // RollbackChanges
	"Discard",                    // Discard
	"Amend",                      // Amend
	"last commit",                // LastCommit
	"Amend last commit",          // AmendLastCommit
	"Commit message",             // CommitMessage
	"Describe the change",        // DescribeTheChange
	"Commit changes",             // CommitChanges
	"Enter a commit message.",    // EnterACommitMessage
	"Unversioned Files",          // UnversionedFiles
	"Previous change",            // PreviousChange
	"Next change",                // NextChange
	"Unified",                    // Unified
	"Default",                    // Default
	"Ignore whitespace",          // IgnoreWhitespace
	"Ignore trailing whitespace", // IgnoreTrailingWhitespace
	"Comparison policy",          // ComparisonPolicy
	"Close diff",                 // CloseDiff
	"Diff content (F7 next, Shift+F7 previous)", // DiffContentF7NextShiftF7Previous
	"Invalid regular expression",                // InvalidRegularExpression
	"Filter",                                    // Filter
	"Text or hash",                              // TextOrHash
	"Regex",                                     // Regex
	"Match case",                                // MatchCase
	"User",                                      // User
	"Filter by author",                          // FilterByAuthor
	"User filter",                               // UserFilter
	"Author contains",                           // AuthorContains
	"Date",                                      // Date
	"Refresh log",                               // RefreshLog
	"Show details",                              // ShowDetails
	"Log options",                               // LogOptions
	"Show hash",                                 // ShowHash
	"Subject",                                   // Subject
	"Hash",                                      // Hash
	"Loading...",                                // LoadingEllipsis
	"Changed files",                             // ChangedFiles
	"Select a commit",                           // SelectACommit
	"Close conflicts",                           // CloseConflicts
	"Select a conflicted file",                  // SelectAConflictedFile
	"Yours content",                             // YoursContent
	"Theirs content",                            // TheirsContent
	"Result",                                    // Result
	"Merge result",                              // MergeResult
	"Change",                                    // Change
	"Conflict",                                  // Conflict
	"Accepted",                                  // Accepted
	"Accept Yours",                              // AcceptYours
	"Accept Theirs",                             // AcceptTheirs
	"Conflict chunks",                           // ConflictChunks
	"Apply non-conflicting changes",             // ApplyNonConflictingChanges
	"Result edited manually",                    // ResultEditedManually
	"Reset result to chunk choices",             // ResetResultToChunkChoices
	"Mark resolved",                             // MarkResolved
	"Upstream",                                  // Upstream
	"Load commits",                              // LoadCommits
	"Start rebase",                              // StartRebase
	"Language",                                  // Language
	"Dark",                                      // Dark
	"Light",                                     // Light
	"All branches",                              // AllBranches
	"Any date",                                  // AnyDate
	"Today",                                     // Today
	"Last 7 days",                               // Last7Days
	"Last 30 days",                              // Last30Days
	"Before today",                              // BeforeToday
	"Staged changes",                            // StagedChanges
	"pick",                                      // Pick
	"reword",                                    // Reword
	"edit",                                      // Edit
	"squash",                                    // Squash
	"fixup",                                     // Fixup
	"drop",                                      // DropText
	func(branch string) string { return "Log: " + branch },                              // LogTitle
	func(name string) string { return "Checkout " + name },                              // CheckoutTitle
	func(name string) string { return "Stash the changes and check out " + name + "?" }, // StashCheckoutPrompt
	func(name string) string { return "From " + name },                                  // FromRef
	func(name string) string { return "Rename " + name },                                // RenameTitle
	func(name string) string { return "Delete " + name },                                // DeleteTitle
	func(name string) string { return "Delete branch " + name + "?" },                   // DeleteBranchPrompt
	func(branch, base string) string {
		return fmt.Sprintf("Branch %s is not fully merged to %s.", branch, base)
	}, // NotFullyMerged
	func(n int) string { return fmt.Sprintf("%d unmerged commits. Delete anyway?", n) },                       // UnmergedCount
	func(url string) string { return "Git credentials for " + url },                                           // CredentialsTitle
	func(name string) string { return name + " (current)" },                                                   // CurrentBranch
	func(name string) string { return "Created " + name },                                                     // CreatedBranch
	func(name string) string { return "Renamed to " + name },                                                  // RenamedBranch
	func(name string) string { return "Deleted " + name },                                                     // DeletedBranch
	func(name string) string { return "Checked out " + name },                                                 // CheckedOut
	func(name string) string { return "Pulled " + name },                                                      // PulledBranch
	func(name string) string { return "Merged " + name },                                                      // MergedBranch
	func(name string) string { return "Rebased onto " + name },                                                // RebasedBranch
	func(name string) string { return "Checked out and rebased " + name },                                     // CheckedOutRebased
	func(name string) string { return "Merge conflicts in " + name },                                          // MergeConflicts
	func(n int) string { return fmt.Sprintf("%d refs", n) },                                                   // RefsCount
	func(ref string) string { return "Drop " + ref + "? This cannot be undone." },                             // DropStashPrompt
	func(ref string) string { return "Pop " + ref + "? The stash is removed if the apply succeeds." },         // PopStashPrompt
	func(path string) string { return "Discard uncommitted changes to " + path + "? This cannot be undone." }, // DiscardPrompt
	func(path string) string { return "Include " + path },                                                     // IncludePath
	func(current, total int) string { return fmt.Sprintf("Change %d of %d", current, total) },                 // ChangePosition
	func(name string) string { return "Author: " + name },                                                     // AuthorName
	func(name string) string { return "Committer: " + name },                                                  // CommitterName
	func(date string) string { return "authored on " + date },                                                 // AuthoredOn
	func(date string) string { return "committed on " + date },                                                // CommittedOn
	func(hashes string) string { return "Parents: " + hashes },                                                // ParentCommits
	func(n int, names string) string { return fmt.Sprintf("In %d branches: %s", n, names) },                   // InBranches
	func(n int) string { return fmt.Sprintf("Action %d", n) },                                                 // ActionNumber
	func(n int) string { return fmt.Sprintf("Message %d", n) },                                                // MessageNumber
	func(n int) string { return fmt.Sprintf("Move up %d", n) },                                                // MoveUp
	func(n int) string { return fmt.Sprintf("Move down %d", n) },                                              // MoveDown
	func(n int) string { return fmt.Sprintf("Accept Yours %d", n) },                                           // AcceptYoursNumber
	func(n int) string { return fmt.Sprintf("Accept Theirs %d", n) },                                          // AcceptTheirsNumber
	func(kind string, n, start, end int) string {
		return fmt.Sprintf("%s %d · base lines %d–%d", kind, n, start, end)
	}, // ChunkDescription
	func(when, now time.Time) string { return relative(when, now, false) }, // RelativeDate
	func(n int) string { return fmt.Sprintf("%d commits", n) },             // CommitsCount
	func(n int) string {
		if n == 1 {
			return "1 file"
		}
		return fmt.Sprintf("%d files", n)
	}, // FilesCount
	func(t time.Time) string { return t.Format("2/1/06, 3:04 pm") }, // ShortDate
}
var ZhCN = Messages{
	"本地变更未能完全恢复；储藏已保留（%s）：%v", // CheckoutRestoreError
	"分支筛选",      // BranchFilter
	"不支持裸仓库",    // BareRepository
	"设置文件无效",    // InvalidSettings
	"未知的设置版本",   // UnknownSettingsVersion
	"成功",        // PushSuccess
	"强制更新",      // PushForced
	"新引用",       // PushNew
	"已拒绝",       // PushRejected
	"已删除",       // PushDeleted
	"已是最新",      // PushUpToDate
	"错误",        // PushError
	"已添加",       // StatusAdded
	"已修改",       // StatusModified
	"已删除",       // StatusDeleted
	"已重命名",      // StatusRenamed
	"已复制",       // StatusCopied
	"未合并",       // StatusUnmerged
	"未版本控制",     // StatusUntracked
	"类型已改变",     // StatusTypeChanged
	"我方（变基分支）",  // YoursRebasing
	"对方（上游）",    // TheirsUpstream
	"变基完成",      // RebaseDone
	"没有可变基的内容",  // RebaseNothing
	"变基因冲突而停止",  // RebaseConflicts
	"变基已暂停以供编辑", // RebaseStopped

	"正在运行 Git…",       // RunningGit
	"正在加载或保存冲突…",      // LoadingConflict
	"差异文件路径无效",        // InvalidDiffPath
	"二进制文件无法显示文本差异",   // BinaryDiff
	"没有可变基的提交",        // NoCommitsRebase
	"变基历史已改变；请重新加载提交", // HistoryChanged
	"变基计划无效。请检查操作和说明；合并与修补需要前置提交，且每次仅支持修改一个提交说明。", // InvalidTodo
	"Git 操作失败",  // OperationFailed
	"不是 Git 仓库", // NotRepository
	"关闭对话框",     // CloseDialog
	"我方",        // Yours
	"对方",        // Theirs
	"基准",        // Base
	"重置",        // Reset
	"还原",        // Revert
	"拣选",        // CherryPick

	"提交",        // Commit
	"提交并推送",     // CommitAndPush
	"提交并推送…",    // CommitAndPushEllipsis
	"推送",        // Push
	"拉取",        // Pull
	"获取",        // Fetch
	"分支",        // Branches
	"冲突",        // Conflicts
	"交互式变基",     // InteractiveRebase
	"差异",        // Diff
	"设置",        // Settings
	"刷新",        // Refresh
	"打开仓库",      // OpenRepository
	"打开储藏",      // OpenStash
	"最近项目",      // RecentProjects
	"最近仓库",      // RecentRepositories
	"外观",        // Appearance
	"主题",        // Theme
	"确定",        // OK
	"打开",        // Open
	"仓库路径",      // RepositoryPath
	"Git 工作区路径", // PathToAGitWorkingTree
	"正在加载仓库…",   // LoadingRepositoryEllipsis
	"提交失败",      // CommitFailed
	"已提交",       // Committed
	"变更",        // Changes
	"已暂存",       // Staged
	"已取消暂存",     // Unstaged
	"已丢弃",       // Discarded
	"正在变基",      // RebaseInProgress
	"继续变基",      // ContinueRebase
	"跳过提交",      // SkipCommit
	"中止变基",      // AbortRebase
	"变基已暂停以供编辑；请修改上次提交，然后继续变基。", // RebaseStoppedForEditAmendTheCommitThenContinueTheRebase
	"取消",           // Cancel
	"检出将覆盖本地变更。",   // LocalChangesWouldBeOverwrittenByCheckout
	"储藏并检出",        // StashAndCheckout
	"新建分支",         // NewBranch
	"分支名称",         // BranchName
	"创建分支",         // CreateBranch
	"请输入分支名称。",     // EnterABranchName
	"新名称",          // NewName
	"重命名",          // Rename
	"删除",           // Delete
	"强制删除",         // ForceDelete
	"推送结果",         // PushResults
	"关闭",           // Close
	"先拉取再推送",       // PullThenPush
	"推送提交",         // PushCommits
	"请选择要推送的分支。",   // SelectABranchToPush
	"没有可推送的内容。",    // NothingToPush
	"远程",           // Remote
	"目标分支",         // TargetBranch
	"没有待推送的提交",     // NoOutgoingCommits
	"使用租约强制推送",     // ForceWithLease
	"推送标签",         // PushTags
	"推送未返回各引用的结果。", // PushProducedNoPerRefResults
	"分支推送被拒绝。是否先拉取再推送？", // ABranchWasRejectedPullThenPushAgain
	"分支",         // Branch
	"更新方式",       // UpdateMethod
	"合并",         // Merge
	"变基",         // Rebase
	"立即拉取",       // PullNow
	"请输入远程名称。",   // EnterARemote
	"本地",         // Local
	"标签",         // Tags
	"获取远程更新",     // FetchRemote
	"拉取变更",       // PullChanges
	"推送分支",       // PushBranches
	"储藏",         // Stash
	"远程",         // Remotes
	"分支树",        // BranchesTree
	"分支操作",       // BranchActions
	"检出",         // Checkout
	"基于此分支新建",    // NewBranchFrom
	"合并到当前分支",    // MergeIntoCurrent
	"将当前分支变基到此处", // RebaseCurrentOnto
	"比较",         // Compare
	"检出并变基",      // CheckoutAndRebase
	"收藏",         // Favorite
	"取消收藏",       // Unfavorite
	"查找分支",       // FindBranch
	"输入以筛选",      // TypeToFilter
	"本地变更未完全恢复",  // LocalChangesNotFullyRestored
	"已保留储藏。请打开“冲突”解决剩余变更，或打开储藏查看已保存的变更。", // TheStashWasKeptOpenConflictsToResolveTheRemainingChangesOrOpenStashToInspectTheSavedChanges
	"获取失败",          // FetchFailed
	"已获取",           // Fetched
	"正在获取",          // Fetching
	"Git 错误",        // GitError
	"已推送",           // Pushed
	"已保存远程设置",       // RemotesSaved
	"已更新储藏",         // StashUpdated
	"Git 凭据",        // GitCredentials
	"用户名",           // Username
	"密码",            // Password
	"储藏",            // Stashes
	"没有储藏。",         // NoStashes
	"创建储藏",          // CreateStash
	"说明",            // Message
	"包含未版本控制的文件",    // IncludeUntracked
	"保留暂存区",         // KeepIndex
	"关闭储藏",          // CloseStash
	"保存储藏",          // SaveStash
	"应用",            // Apply
	"弹出",            // Pop
	"删除",            // Drop
	"删除储藏",          // DropStash
	"弹出储藏",          // PopStash
	"立即弹出",          // PopNow
	"名称",            // Name
	"URL",           // URL
	"移除",            // Remove
	"添加远程",          // AddRemote
	"新远程名称",         // NewRemoteName
	"新远程 URL",       // NewRemoteURL
	"保存",            // Save
	"新远程需要名称和 URL。", // ANewRemoteNeedsANameAndAURL
	"远程名称不能为空。",     // RemoteNameCannotBeEmpty
	"没有可提交的内容。请暂存变更或勾选文件。", // NothingToCommitStageAChangeOrIncludeAFile
	"GPG 无法为提交签名。",         // GPGCouldNotSignTheCommit
	"提交前必须解决尚未合并的文件。",      // UnmergedFilesMustBeResolvedBeforeCommitting
	"操作因合并冲突而停止。",          // TheOperationStoppedOnAMergeConflict
	"该提交已应用。",              // ThatCommitIsAlreadyApplied
	"本地变更将被覆盖。",            // LocalChangesWouldBeOverwritten
	"未版本控制的文件将被覆盖。",        // UntrackedFilesWouldBeOverwritten
	"该分支已在另一个工作区中检出。",      // TheBranchIsAlreadyCheckedOutInAnotherWorktree
	"刷新变更",     // RefreshChanges
	"提交选项",     // CommitOptions
	"添加签署信息",   // SignOff
	"作者",       // Author
	"姓名 <邮箱>",  // NameEmail
	"没有变更",     // NoChanges
	"暂存",       // Stage
	"取消暂存",     // Unstage
	"回滚",       // Rollback
	"显示差异",     // ShowDiff
	"回滚变更",     // RollbackChanges
	"丢弃",       // Discard
	"修改",       // Amend
	"上次提交",     // LastCommit
	"修改上次提交",   // AmendLastCommit
	"提交说明",     // CommitMessage
	"描述此次变更",   // DescribeTheChange
	"提交变更",     // CommitChanges
	"请输入提交说明。", // EnterACommitMessage
	"未版本控制的文件", // UnversionedFiles
	"上一个变更",    // PreviousChange
	"下一个变更",    // NextChange
	"统一视图",     // Unified
	"默认",       // Default
	"忽略空白",     // IgnoreWhitespace
	"忽略行尾空白",   // IgnoreTrailingWhitespace
	"比较策略",     // ComparisonPolicy
	"关闭差异",     // CloseDiff
	"差异内容（F7 下一个，Shift+F7 上一个）", // DiffContentF7NextShiftF7Previous
	"正则表达式无效",                   // InvalidRegularExpression
	"筛选",                        // Filter
	"文本或哈希",                     // TextOrHash
	"正则表达式",                     // Regex
	"区分大小写",                     // MatchCase
	"用户",                        // User
	"按作者筛选",                     // FilterByAuthor
	"用户筛选",                      // UserFilter
	"作者包含",                      // AuthorContains
	"日期",                        // Date
	"刷新日志",                      // RefreshLog
	"显示详情",                      // ShowDetails
	"日志选项",                      // LogOptions
	"显示哈希",                      // ShowHash
	"主题",                        // Subject
	"哈希",                        // Hash
	"正在加载…",                     // LoadingEllipsis
	"变更文件",                      // ChangedFiles
	"选择一个提交",                    // SelectACommit
	"关闭冲突",                      // CloseConflicts
	"选择一个冲突文件",                  // SelectAConflictedFile
	"我方内容",                      // YoursContent
	"对方内容",                      // TheirsContent
	"结果",                        // Result
	"合并结果",                      // MergeResult
	"变更",                        // Change
	"冲突",                        // Conflict
	"已接受",                       // Accepted
	"接受我方",                      // AcceptYours
	"接受对方",                      // AcceptTheirs
	"冲突块",                       // ConflictChunks
	"应用无冲突的变更",                  // ApplyNonConflictingChanges
	"结果已手动编辑",                   // ResultEditedManually
	"按块选择重置结果",                  // ResetResultToChunkChoices
	"标记为已解决",                    // MarkResolved
	"上游",                        // Upstream
	"加载提交",                      // LoadCommits
	"开始变基",                      // StartRebase
	"语言",                        // Language
	"深色",                        // Dark
	"浅色",                        // Light
	"所有分支",                      // AllBranches
	"不限日期",                      // AnyDate
	"今天",                        // Today
	"最近 7 天",                    // Last7Days
	"最近 30 天",                   // Last30Days
	"今天之前",                      // BeforeToday
	"已暂存的变更",                    // StagedChanges
	"保留",                        // Pick
	"修改说明",                      // Reword
	"编辑",                        // Edit
	"合并",                        // Squash
	"修补",                        // Fixup
	"丢弃",                        // DropText
	func(branch string) string { return "日志：" + branch },          // LogTitle
	func(name string) string { return "检出 " + name },              // CheckoutTitle
	func(name string) string { return "是否储藏变更并检出 " + name + "？" }, // StashCheckoutPrompt
	func(name string) string { return "起点：" + name },              // FromRef
	func(name string) string { return "重命名 " + name },             // RenameTitle
	func(name string) string { return "删除 " + name },              // DeleteTitle
	func(name string) string { return "是否删除分支 " + name + "？" },    // DeleteBranchPrompt
	func(branch, base string) string {
		return fmt.Sprintf("分支 %s 尚未完全合并到 %s。", branch, base)
	}, // NotFullyMerged
	func(n int) string { return fmt.Sprintf("有 %d 个尚未合并的提交。仍要删除吗？", n) }, // UnmergedCount
	func(url string) string { return "Git 凭据：" + url },                   // CredentialsTitle
	func(name string) string { return name + "（当前）" },                    // CurrentBranch
	func(name string) string { return "已创建 " + name },                    // CreatedBranch
	func(name string) string { return "已重命名为 " + name },                  // RenamedBranch
	func(name string) string { return "已删除 " + name },                    // DeletedBranch
	func(name string) string { return "已检出 " + name },                    // CheckedOut
	func(name string) string { return "已拉取 " + name },                    // PulledBranch
	func(name string) string { return "已合并 " + name },                    // MergedBranch
	func(name string) string { return "已变基到 " + name },                   // RebasedBranch
	func(name string) string { return "已检出并变基 " + name },                 // CheckedOutRebased
	func(name string) string { return "合并存在冲突：" + name },                 // MergeConflicts
	func(n int) string { return fmt.Sprintf("%d 个引用", n) },               // RefsCount
	func(ref string) string { return "是否删除 " + ref + "？此操作无法撤销。" },       // DropStashPrompt
	func(ref string) string { return "是否弹出 " + ref + "？应用成功后将删除该储藏。" },   // PopStashPrompt
	func(path string) string {
		return "是否丢弃 " + path + " 中尚未提交的变更？此操作无法撤销。"
	}, // DiscardPrompt
	func(path string) string { return "包含 " + path },                                        // IncludePath
	func(current, total int) string { return fmt.Sprintf("第 %d / %d 个变更", current, total) }, // ChangePosition
	func(name string) string { return "作者：" + name },                                        // AuthorName
	func(name string) string { return "提交者：" + name },                                       // CommitterName
	func(date string) string { return "创作于 " + date },                                       // AuthoredOn
	func(date string) string { return "提交于 " + date },                                       // CommittedOn
	func(hashes string) string { return "父提交：" + hashes },                                   // ParentCommits
	func(n int, names string) string { return fmt.Sprintf("位于 %d 个分支：%s", n, names) },       // InBranches
	func(n int) string { return fmt.Sprintf("操作 %d", n) },                                   // ActionNumber
	func(n int) string { return fmt.Sprintf("说明 %d", n) },                                   // MessageNumber
	func(n int) string { return fmt.Sprintf("上移 %d", n) },                                   // MoveUp
	func(n int) string { return fmt.Sprintf("下移 %d", n) },                                   // MoveDown
	func(n int) string { return fmt.Sprintf("接受我方 %d", n) },                                 // AcceptYoursNumber
	func(n int) string { return fmt.Sprintf("接受对方 %d", n) },                                 // AcceptTheirsNumber
	func(kind string, n, start, end int) string {
		return fmt.Sprintf("%s %d · 基准行 %d–%d", kind, n, start, end)
	}, // ChunkDescription
	func(when, now time.Time) string { return relative(when, now, true) }, // RelativeDate
	func(n int) string { return fmt.Sprintf("%d 个提交", n) },                // CommitsCount
	func(n int) string { return fmt.Sprintf("%d 个文件", n) },                // FilesCount
	func(t time.Time) string { return t.Format("2006/1/2 15:04") },        // ShortDate
}

// ForLanguage resolves a persisted choice. Chinese is the default.
func ForLanguage(language string) *Messages {
	if language == "en" {
		return &En
	}
	return &ZhCN
}

// StartupLanguage permits deterministic automation without changing saved settings.
func StartupLanguage(saved string) string {
	if v := os.Getenv("R_GIT_LANG"); v == "en" || v == "zh-CN" {
		return v
	}
	return saved
}

// Locale is owned by a view. The shell binds it once at startup and on changes.
// A standalone view lazily binds once, which also supports isolated driver tests.
type Locale struct{ active *atomic.Pointer[Messages] }

// UseMessages publishes a catalogue atomically because Git workers can report
// messages while the window thread changes language.
func (l *Locale) UseMessages(messages *Messages) {
	if l.active == nil {
		l.active = new(atomic.Pointer[Messages])
	}
	l.active.Store(messages)
}
func (l *Locale) Text() *Messages {
	if l.active != nil {
		if messages := l.active.Load(); messages != nil {
			return messages
		}
	}
	return ForLanguage(StartupLanguage("zh-CN"))
}

func relative(when, now time.Time, zh bool) string {
	d := now.Sub(when)
	if d < time.Minute {
		if zh {
			return "刚刚"
		}
		return "just now"
	}
	if d < time.Hour {
		if zh {
			return fmt.Sprintf("%d 分钟前", int(d/time.Minute))
		}
		return fmt.Sprintf("%d minutes ago", int(d/time.Minute))
	}
	if d < 24*time.Hour {
		if zh {
			return fmt.Sprintf("%d 小时前", int(d/time.Hour))
		}
		return fmt.Sprintf("%d hours ago", int(d/time.Hour))
	}
	if d < 7*24*time.Hour {
		if zh {
			return fmt.Sprintf("%d 天前", int(d/(24*time.Hour)))
		}
		return fmt.Sprintf("%d days ago", int(d/(24*time.Hour)))
	}
	if zh {
		return when.Format("2006/1/2 15:04")
	}
	return when.Format("2/1/06, 3:04 pm")
}

// Display methods translate only known enum choices, never names supplied by Git.
func (m *Messages) BranchChoice(value string) string {
	if value == "All branches" {
		return m.AllBranches
	}
	return value
}
func (m *Messages) DateChoice(value string) string {
	switch value {
	case "Any date":
		return m.AnyDate
	case "Today":
		return m.Today
	case "Last 7 days":
		return m.Last7Days
	case "Last 30 days":
		return m.Last30Days
	case "Before today":
		return m.BeforeToday
	}
	return value
}
func (m *Messages) ActionChoice(value string) string {
	switch value {
	case "pick":
		return m.Pick
	case "reword":
		return m.Reword
	case "edit":
		return m.Edit
	case "squash":
		return m.Squash
	case "fixup":
		return m.Fixup
	case "drop":
		return m.DropText
	}
	return value
}
func (m *Messages) PolicyChoice(value string) string {
	switch value {
	case "Default":
		return m.Default
	case "Ignore whitespace":
		return m.IgnoreWhitespace
	case "Ignore trailing whitespace":
		return m.IgnoreTrailingWhitespace
	}
	return value
}
func (m *Messages) ThemeChoice(value string) string {
	if m == &En {
		return value
	}
	if value == "light" {
		return m.Light
	}
	return m.Dark
}
func LanguageChoice(value string) string {
	if value == "en" {
		return "English"
	}
	return "简体中文"
}

func (m *Messages) FileStatus(value string) string {
	switch value {
	case "added":
		return m.StatusAdded
	case "modified":
		return m.StatusModified
	case "deleted":
		return m.StatusDeleted
	case "renamed":
		return m.StatusRenamed
	case "copied":
		return m.StatusCopied
	case "unmerged":
		return m.StatusUnmerged
	case "untracked":
		return m.StatusUntracked
	case "type-changed":
		return m.StatusTypeChanged
	}
	return value
}
func (m *Messages) ConflictSide(value string) string {
	switch value {
	case "Yours":
		return m.Yours
	case "Theirs":
		return m.Theirs
	case "Yours (rebasing branch)":
		return m.YoursRebasing
	case "Theirs (upstream)":
		return m.TheirsUpstream
	}
	return value
}

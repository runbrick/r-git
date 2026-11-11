package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// UpdateMethod is how a rejected or explicit update brings remote commits
// onto the current branch. Values match git4idea.config.UpdateMethod.
type UpdateMethod string

const (
	// UpdateMerge merges fetched commits. Java default:
	// plugins/git4idea/shared/src/git4idea/config/GitVcsOptions.kt:30
	// (var updateMethod: UpdateMethod by enum(UpdateMethod.MERGE)).
	UpdateMerge UpdateMethod = "merge"
	// UpdateRebase rebases local commits onto the fetched branch.
	UpdateRebase UpdateMethod = "rebase"
	// UpdateBranchDefault uses the branch's own pull.rebase config.
	UpdateBranchDefault UpdateMethod = "branch-default"
)

// AppSettings is application-wide state. It is stored once, not per repo.
type AppSettings struct {
	// ShowLogInEditor keeps the log as the main view. Java default true:
	// platform/vcs-log/impl/src/com/intellij/vcs/log/impl/VcsLogApplicationSettings.kt:146
	// (var showInEditor = true), bound to CommonUiProperties.SHOW_IN_EDITOR
	// ("Window.ShowInEditor"). The UI decides later how to honor it.
	ShowLogInEditor bool `json:"showLogInEditor"`

	// MaxRecentProjects caps the recent list. Java default 50:
	// platform/platform-impl/resources/intellij.platform.ide.impl.xml:1522
	// (advancedSetting id="ide.max.recent.projects" default="50").
	MaxRecentProjects int `json:"maxRecentProjects"`

	// Recent is most-recent-first.
	Recent []RecentProject `json:"recent"`

	// Git holds the application-scoped git options a standalone client uses.
	Git AppGitSettings `json:"git"`
}

// AppGitSettings ports the subset of GitVcsApplicationSettings.State that
// a standalone client needs.
type AppGitSettings struct {
	// PathToGit is the git executable. Empty means auto-detect, matching
	// State.myPathToGit = null in
	// plugins/git4idea/backend/src/config/GitVcsApplicationSettings.java:25.
	PathToGit string `json:"pathToGit"`

	// UseCredentialHelper defaults false:
	// GitVcsApplicationSettings.java:29 (USE_CREDENTIAL_HELPER = false).
	UseCredentialHelper bool `json:"useCredentialHelper"`
}

// RepoSettings is the per-repository subset of GitVcsOptions and
// GitSharedSettings. Fields the Java IDE stores for popups, annotate,
// branch widgets, or multi-root sync are listed as skipped in the PR.
type RepoSettings struct {
	// PathToGit overrides the application path for this repo. Empty means
	// use the application value (itself empty means auto-detect).
	// GitVcsOptions.kt:18 (pathToGit: String? by string()).
	PathToGit string `json:"pathToGit"`

	// UpdateMethod default merge: GitVcsOptions.kt:30.
	UpdateMethod UpdateMethod `json:"updateMethod"`

	// PushAutoUpdate updates the branch and retries when a push is rejected.
	// Default false: GitVcsOptions.kt:44 (isPushAutoUpdate: Boolean by property(false)).
	PushAutoUpdate bool `json:"pushAutoUpdate"`

	// SignOffCommit adds Signed-off-by to new commits.
	// Default false: GitVcsOptions.kt:98 (isSignOffCommit: Boolean by property(false)).
	SignOffCommit bool `json:"signOffCommit"`

	// ForcePushProhibitedPatterns hides force-push for matching branch names.
	// Default master and main:
	// plugins/git4idea/backend/src/config/GitSharedSettings.java:35
	// (FORCE_PUSH_PROHIBITED_PATTERNS = List.of(MASTER, MAIN)).
	// Matching is a full-string match of each pattern, same as
	// GitSharedSettings.isMatchSafe (branch.matches("^" + pattern + "$")).
	ForcePushProhibitedPatterns []string `json:"forcePushProhibitedPatterns"`
}

// RecentProject is one entry in the recent-projects list. Java stores the
// same facts on RecentProjectMetaInfo (displayName, activationTimestamp)
// keyed by recentPaths order:
// platform/platform-impl/src/com/intellij/ide/RecentProjectMetaInfo.kt:30
// and :48.
type RecentProject struct {
	Path        string `json:"path"`
	DisplayName string `json:"displayName"`
	LastOpened  int64  `json:"lastOpened"`
}

// DefaultAppSettings returns Java defaults for a new installation.
func DefaultAppSettings() AppSettings {
	return AppSettings{
		ShowLogInEditor:   true,
		MaxRecentProjects: 50,
		Recent:            []RecentProject{},
		Git:               AppGitSettings{},
	}
}

// DefaultRepoSettings returns Java defaults for a repository that has no
// saved settings yet.
func DefaultRepoSettings() RepoSettings {
	return RepoSettings{
		UpdateMethod:                UpdateMerge,
		ForcePushProhibitedPatterns: []string{"master", "main"},
	}
}

// Normalize fills zero-value fields that JSON would otherwise leave blank
// when an older writer omitted them. Callers that Load get this applied.
func (s AppSettings) normalized() AppSettings {
	if s.MaxRecentProjects < 1 {
		s.MaxRecentProjects = DefaultAppSettings().MaxRecentProjects
	}
	if s.Recent == nil {
		s.Recent = []RecentProject{}
	}
	return s
}

func (s RepoSettings) normalized() RepoSettings {
	if s.UpdateMethod == "" {
		s.UpdateMethod = UpdateMerge
	}
	if s.ForcePushProhibitedPatterns == nil {
		s.ForcePushProhibitedPatterns = []string{"master", "main"}
	}
	return s
}

// AppPath is the application settings file inside dir.
func AppPath(dir string) string {
	return filepath.Join(dir, "settings.json")
}

// RepoPath is the settings file for a repository. The file name is a hash
// of the absolute, cleaned path so two paths that name the same directory
// share one file and the user's repository is never written.
func RepoPath(dir, repoPath string) (string, error) {
	key, err := RepoKey(repoPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repos", key+".json"), nil
}

// RepoKey hashes the absolute, cleaned repository path.
func RepoKey(repoPath string) (string, error) {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("config: repo path: %w", err)
	}
	cleaned := filepath.Clean(abs)
	sum := sha256.Sum256([]byte(cleaned))
	return hex.EncodeToString(sum[:]), nil
}

// LoadApp reads application settings. A missing file returns defaults.
func LoadApp(dir string) (AppSettings, error) {
	value, err := Load(AppPath(dir), DefaultAppSettings, migrateAppV0)
	if err != nil {
		return AppSettings{}, err
	}
	return value.normalized(), nil
}

// SaveApp writes application settings atomically.
func SaveApp(dir string, settings AppSettings) error {
	return Save(AppPath(dir), settings.normalized())
}

// LoadRepo reads settings for one repository. A missing file returns defaults.
func LoadRepo(dir, repoPath string) (RepoSettings, error) {
	path, err := RepoPath(dir, repoPath)
	if err != nil {
		return RepoSettings{}, err
	}
	value, err := Load(path, DefaultRepoSettings, migrateRepoV0)
	if err != nil {
		return RepoSettings{}, err
	}
	return value.normalized(), nil
}

// SaveRepo writes settings for one repository atomically.
func SaveRepo(dir, repoPath string, settings RepoSettings) error {
	path, err := RepoPath(dir, repoPath)
	if err != nil {
		return err
	}
	return Save(path, settings.normalized())
}

// TouchRecent records an open. The entry moves to the front, a previous
// entry for the same cleaned path is dropped, and the list is cut to
// MaxRecentProjects. DisplayName falls back to the directory base name,
// matching getProjectNameOnlyByPath in RecentProjectsManagerBase.kt.
// LastOpened is a Unix millisecond timestamp, the unit Java stores in
// RecentProjectMetaInfo.activationTimestamp.
func (s *AppSettings) TouchRecent(repoPath, displayName string, lastOpenedUnixMilli int64) error {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("config: recent path: %w", err)
	}
	cleaned := filepath.Clean(abs)
	if displayName == "" {
		displayName = filepath.Base(cleaned)
	}
	next := []RecentProject{{
		Path:        cleaned,
		DisplayName: displayName,
		LastOpened:  lastOpenedUnixMilli,
	}}
	for _, item := range s.Recent {
		if samePath(item.Path, cleaned) {
			continue
		}
		next = append(next, item)
	}
	limit := s.MaxRecentProjects
	if limit < 1 {
		limit = DefaultAppSettings().MaxRecentProjects
	}
	if len(next) > limit {
		next = next[:limit]
	}
	s.Recent = next
	return nil
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

type appV0 struct {
	ShowLogInEditor   *bool           `json:"showLogInEditor"`
	MaxRecentProjects *int            `json:"maxRecentProjects"`
	Recent            []RecentProject `json:"recent"`
	Git               *AppGitSettings `json:"git"`
}

func migrateAppV0(raw json.RawMessage) (AppSettings, error) {
	var old appV0
	if err := json.Unmarshal(raw, &old); err != nil {
		return AppSettings{}, fmt.Errorf("%w: %s", ErrInvalidSettings, err)
	}
	settings := DefaultAppSettings()
	if old.ShowLogInEditor != nil {
		settings.ShowLogInEditor = *old.ShowLogInEditor
	}
	if old.MaxRecentProjects != nil && *old.MaxRecentProjects >= 1 {
		settings.MaxRecentProjects = *old.MaxRecentProjects
	}
	if old.Recent != nil {
		settings.Recent = old.Recent
	}
	if old.Git != nil {
		settings.Git = *old.Git
	}
	return settings, nil
}

type repoV0 struct {
	PathToGit                   *string       `json:"pathToGit"`
	UpdateMethod                *UpdateMethod `json:"updateMethod"`
	PushAutoUpdate              *bool         `json:"pushAutoUpdate"`
	SignOffCommit               *bool         `json:"signOffCommit"`
	ForcePushProhibitedPatterns []string      `json:"forcePushProhibitedPatterns"`
}

func migrateRepoV0(raw json.RawMessage) (RepoSettings, error) {
	var old repoV0
	if err := json.Unmarshal(raw, &old); err != nil {
		return RepoSettings{}, fmt.Errorf("%w: %s", ErrInvalidSettings, err)
	}
	settings := DefaultRepoSettings()
	if old.PathToGit != nil {
		settings.PathToGit = *old.PathToGit
	}
	if old.UpdateMethod != nil && *old.UpdateMethod != "" {
		settings.UpdateMethod = *old.UpdateMethod
	}
	if old.PushAutoUpdate != nil {
		settings.PushAutoUpdate = *old.PushAutoUpdate
	}
	if old.SignOffCommit != nil {
		settings.SignOffCommit = *old.SignOffCommit
	}
	if old.ForcePushProhibitedPatterns != nil {
		settings.ForcePushProhibitedPatterns = old.ForcePushProhibitedPatterns
	}
	return settings, nil
}

// ValidUpdateMethod reports whether value is one of the ported methods.
func ValidUpdateMethod(value UpdateMethod) bool {
	switch value {
	case UpdateMerge, UpdateRebase, UpdateBranchDefault:
		return true
	default:
		return false
	}
}

// GitExecutable returns the git binary path to use. A repo override wins,
// then the application path. Empty means the caller should look up git
// on PATH. Neither value is resolved here.
func GitExecutable(app AppSettings, repo RepoSettings) string {
	if path := strings.TrimSpace(repo.PathToGit); path != "" {
		return path
	}
	return strings.TrimSpace(app.Git.PathToGit)
}

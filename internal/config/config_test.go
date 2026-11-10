package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDirUsesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDir, dir)
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(dir) {
		t.Fatalf("Dir() = %q, want %q", got, filepath.Clean(dir))
	}
}

func TestDirDefaultContainsAppName(t *testing.T) {
	t.Setenv(EnvDir, "")
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "r-git" {
		t.Fatalf("Dir() = %q, want base r-git", got)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(base, "r-git") {
		t.Fatalf("Dir() = %q, want %q", got, filepath.Join(base, "r-git"))
	}
}

func TestRoundTripAppAndRepo(t *testing.T) {
	dir := t.TempDir()
	app := DefaultAppSettings()
	app.ShowLogInEditor = false
	app.Git.PathToGit = `C:\Git\bin\git.exe`
	app.Git.UseCredentialHelper = true
	if err := app.TouchRecent(filepath.Join(dir, "repo"), "Demo", 100); err != nil {
		t.Fatal(err)
	}
	if err := SaveApp(dir, app); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, app.normalized()) {
		t.Fatalf("app round trip\n got %#v\nwant %#v", loaded, app.normalized())
	}

	repo := DefaultRepoSettings()
	repo.PathToGit = "/usr/local/bin/git"
	repo.UpdateMethod = UpdateRebase
	repo.PushAutoUpdate = true
	repo.SignOffCommit = true
	repo.ForcePushProhibitedPatterns = []string{"release/.*"}
	repoPath := filepath.Join(dir, "repo")
	if err := SaveRepo(dir, repoPath, repo); err != nil {
		t.Fatal(err)
	}
	gotRepo, err := LoadRepo(dir, repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotRepo, repo.normalized()) {
		t.Fatalf("repo round trip\n got %#v\nwant %#v", gotRepo, repo.normalized())
	}

	raw, err := os.ReadFile(AppPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var env fileEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.Version != 1 {
		t.Fatalf("version = %d, want 1", env.Version)
	}
}

func TestMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	app, err := LoadApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(app, DefaultAppSettings()) {
		t.Fatalf("missing app = %#v", app)
	}
	repo, err := LoadRepo(dir, filepath.Join(dir, "nowhere"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo, DefaultRepoSettings()) {
		t.Fatalf("missing repo = %#v", repo)
	}
}

func TestDefaultsMatchJava(t *testing.T) {
	app := DefaultAppSettings()
	if !app.ShowLogInEditor {
		t.Fatal("ShowLogInEditor default wants true (VcsLogApplicationSettings.kt:146)")
	}
	if app.MaxRecentProjects != 50 {
		t.Fatalf("MaxRecentProjects = %d, want 50 (intellij.platform.ide.impl.xml:1522)", app.MaxRecentProjects)
	}
	if app.Git.PathToGit != "" || app.Git.UseCredentialHelper {
		t.Fatalf("app git defaults = %#v", app.Git)
	}
	repo := DefaultRepoSettings()
	if repo.UpdateMethod != UpdateMerge {
		t.Fatalf("UpdateMethod = %q, want merge (GitVcsOptions.kt:30)", repo.UpdateMethod)
	}
	if repo.PushAutoUpdate || repo.SignOffCommit || repo.PathToGit != "" {
		t.Fatalf("repo defaults = %#v", repo)
	}
	wantPatterns := []string{"master", "main"}
	if !reflect.DeepEqual(repo.ForcePushProhibitedPatterns, wantPatterns) {
		t.Fatalf("patterns = %#v, want %#v (GitSharedSettings.java:35)", repo.ForcePushProhibitedPatterns, wantPatterns)
	}
}

func TestRepoKeyUsesAbsoluteCleanedPath(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := RepoKey(repo)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RepoKey(repo + string(filepath.Separator) + ".")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("keys differ for the same directory: %s vs %s", a, b)
	}
	other, err := RepoKey(filepath.Join(dir, "other"))
	if err != nil {
		t.Fatal(err)
	}
	if a == other {
		t.Fatal("different paths produced the same key")
	}
}

func TestSaveIsAtomicOnFailure(t *testing.T) {
	dir := t.TempDir()
	original := []byte("{\n  \"version\": 1,\n  \"data\": {}\n}\n")
	if err := os.WriteFile(AppPath(dir), original, 0o644); err != nil {
		t.Fatal(err)
	}

	// A directory sitting where the destination file should be makes
	// os.Rename fail. Save must leave the previous file alone and remove
	// its temporary file.
	blocked := filepath.Join(dir, "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "settings.json", "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Save(filepath.Join(blocked, "settings.json"), DefaultAppSettings())
	if err == nil {
		t.Fatal("Save succeeded; want a rename failure")
	}
	got, readErr := os.ReadFile(AppPath(dir))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("original file changed:\n%s", got)
	}
	left, globErr := filepath.Glob(filepath.Join(blocked, "settings.json.tmp-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestSaveReplacesOnlyAfterRename(t *testing.T) {
	dir := t.TempDir()
	path := AppPath(dir)
	if err := SaveApp(dir, DefaultAppSettings()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := DefaultAppSettings()
	changed.ShowLogInEditor = false
	if err := SaveApp(dir, changed); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) == string(after) {
		t.Fatal("second save did not replace the file")
	}
	loaded, err := LoadApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ShowLogInEditor {
		t.Fatal("reloaded settings kept the old value")
	}
}

func TestMigrateAppV0(t *testing.T) {
	dir := t.TempDir()
	fixture := `{
  "showLogInEditor": false,
  "maxRecentProjects": 3,
  "recent": [
    {"path": "D:/work/old", "displayName": "old", "lastOpened": 9}
  ],
  "git": {"pathToGit": "git", "useCredentialHelper": true}
}`
	if err := os.WriteFile(AppPath(dir), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.ShowLogInEditor || got.MaxRecentProjects != 3 || !got.Git.UseCredentialHelper || got.Git.PathToGit != "git" {
		t.Fatalf("migrated app = %#v", got)
	}
	if len(got.Recent) != 1 || got.Recent[0].DisplayName != "old" || got.Recent[0].LastOpened != 9 {
		t.Fatalf("migrated recent = %#v", got.Recent)
	}
}

func TestMigrateRepoV0(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	path, err := RepoPath(dir, repoPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := `{
  "pathToGit": "/opt/git",
  "updateMethod": "rebase",
  "pushAutoUpdate": true,
  "signOffCommit": true,
  "forcePushProhibitedPatterns": ["main"]
}`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRepo(dir, repoPath)
	if err != nil {
		t.Fatal(err)
	}
	want := RepoSettings{
		PathToGit:                   "/opt/git",
		UpdateMethod:                UpdateRebase,
		PushAutoUpdate:              true,
		SignOffCommit:               true,
		ForcePushProhibitedPatterns: []string{"main"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated repo\n got %#v\nwant %#v", got, want)
	}
}

func TestMigrateV0OmitsKeepDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(AppPath(dir), []byte(`{"recent":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, DefaultAppSettings()) {
		t.Fatalf("sparse v0 = %#v, want defaults", got)
	}
}

func TestUnknownVersion(t *testing.T) {
	dir := t.TempDir()
	body := []byte(`{"version": 99, "data": {}}`)
	if err := os.WriteFile(AppPath(dir), body, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadApp(dir)
	if !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("err = %v, want ErrUnknownVersion", err)
	}
}

func TestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(AppPath(dir), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadApp(dir)
	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("err = %v, want ErrInvalidSettings", err)
	}
}

func TestRecentOrderDedupAndMax(t *testing.T) {
	dir := t.TempDir()
	settings := DefaultAppSettings()
	settings.MaxRecentProjects = 3
	paths := []string{
		filepath.Join(dir, "a"),
		filepath.Join(dir, "b"),
		filepath.Join(dir, "c"),
		filepath.Join(dir, "a"),
		filepath.Join(dir, "d"),
	}
	for i, path := range paths {
		if err := settings.TouchRecent(path, "", int64(i+1)); err != nil {
			t.Fatal(err)
		}
	}
	if len(settings.Recent) != 3 {
		t.Fatalf("len = %d, want 3: %#v", len(settings.Recent), settings.Recent)
	}
	abs := func(name string) string {
		path, err := filepath.Abs(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Clean(path)
	}
	got := []string{settings.Recent[0].Path, settings.Recent[1].Path, settings.Recent[2].Path}
	want := []string{abs("d"), abs("a"), abs("c")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order\n got %v\nwant %v", got, want)
	}
	if settings.Recent[1].LastOpened != 4 {
		t.Fatalf("deduped timestamp = %d, want 4", settings.Recent[1].LastOpened)
	}
	if settings.Recent[0].DisplayName != "d" {
		t.Fatalf("display name = %q, want directory base", settings.Recent[0].DisplayName)
	}

	// A second open of the same path with a trailing separator stays one entry.
	again := abs("d") + string(filepath.Separator)
	if err := settings.TouchRecent(again, "Dee", 9); err != nil {
		t.Fatal(err)
	}
	if len(settings.Recent) != 3 || settings.Recent[0].DisplayName != "Dee" || settings.Recent[0].LastOpened != 9 {
		t.Fatalf("dedup failed: %#v", settings.Recent)
	}
}

func TestRecentPersistsOrder(t *testing.T) {
	dir := t.TempDir()
	settings := DefaultAppSettings()
	settings.MaxRecentProjects = 2
	if err := settings.TouchRecent(filepath.Join(dir, "one"), "One", 1); err != nil {
		t.Fatal(err)
	}
	if err := settings.TouchRecent(filepath.Join(dir, "two"), "Two", 2); err != nil {
		t.Fatal(err)
	}
	if err := settings.TouchRecent(filepath.Join(dir, "three"), "Three", 3); err != nil {
		t.Fatal(err)
	}
	if err := SaveApp(dir, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Recent) != 2 || loaded.Recent[0].DisplayName != "Three" || loaded.Recent[1].DisplayName != "Two" {
		t.Fatalf("persisted recent = %#v", loaded.Recent)
	}
}

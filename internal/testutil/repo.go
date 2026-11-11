package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type Repo struct {
	Dir          string
	t            *testing.T
	command      string
	home         string
	globalConfig string
	systemConfig string
}

func NewRepo(t *testing.T) *Repo {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not on PATH")
	}

	root := t.TempDir()
	repo := &Repo{
		Dir:          filepath.Join(root, "repo"),
		t:            t,
		home:         filepath.Join(root, "home"),
		globalConfig: filepath.Join(root, "gitconfig-global"),
		systemConfig: filepath.Join(root, "gitconfig-system"),
	}
	for _, dir := range []string{repo.Dir, repo.home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{repo.globalConfig, repo.systemConfig} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", repo.home)
	t.Setenv("USERPROFILE", repo.home)
	t.Setenv("GIT_CONFIG_GLOBAL", repo.globalConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", repo.systemConfig)
	repo.command = git

	repo.Git("init", "-q")
	repo.Git("config", "user.name", "Test User")
	repo.Git("config", "user.email", "test@example.invalid")
	return repo
}

func (r *Repo) Git(args ...string) string {
	cmd := exec.Command(r.command, args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(), "HOME="+r.home, "USERPROFILE="+r.home, "GIT_CONFIG_GLOBAL="+r.globalConfig, "GIT_CONFIG_SYSTEM="+r.systemConfig)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s failed: %v\n%s", argsString(args), err, output)
	}
	return string(output)
}

func (r *Repo) WriteFile(path, content string) {
	fullPath := filepath.Join(r.Dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *Repo) Commit(msg string) string {
	r.Git("add", "--all")
	r.Git("commit", "-q", "-m", msg)
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

func argsString(args []string) string {
	return strings.Join(args, " ")
}

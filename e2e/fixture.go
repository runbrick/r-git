//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a throwaway git repo with a local bare remote and isolated config.
type fixture struct {
	root   string
	repo   string
	bare   string
	home   string
	config string
	gitBin string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not on PATH")
	}
	root := t.TempDir()
	f := &fixture{
		root:   root,
		repo:   filepath.Join(root, "repo"),
		bare:   filepath.Join(root, "remote.git"),
		home:   filepath.Join(root, "home"),
		config: filepath.Join(root, "config"),
		gitBin: git,
	}
	for _, dir := range []string{f.repo, f.home, f.config} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"gitconfig-global", "gitconfig-system"} {
		if err := os.WriteFile(filepath.Join(f.home, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f.run(f.root, "init", "-q", f.repo)
	f.run(f.repo, "config", "user.name", "Test User")
	f.run(f.repo, "config", "user.email", "test@example.invalid")
	return f
}

func (f *fixture) run(dir string, args ...string) string {
	cmd := exec.Command(f.gitBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HOME="+f.home,
		"USERPROFILE="+f.home,
		"GIT_CONFIG_GLOBAL="+filepath.Join(f.home, "gitconfig-global"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(f.home, "gitconfig-system"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(cmdError{args: args, err: err, out: string(out)})
	}
	return string(out)
}

type cmdError struct {
	args []string
	err  error
	out  string
}

func (e cmdError) Error() string {
	return "git " + strings.Join(e.args, " ") + ": " + e.err.Error() + "\n" + e.out
}

func (f *fixture) git(t *testing.T, args ...string) string {
	t.Helper()
	return f.gitIn(t, f.repo, args...)
}

func (f *fixture) gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			if err, ok := recovered.(cmdError); ok {
				t.Fatalf("%s", err.Error())
			}
			panic(recovered)
		}
	}()
	return f.run(dir, args...)
}

func (f *fixture) write(name, body string) {
	path := filepath.Join(f.repo, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		panic(err)
	}
}

func (f *fixture) commit(t *testing.T, message string) {
	t.Helper()
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", message)
}

func (f *fixture) addBareRemote(t *testing.T) string {
	t.Helper()
	f.git(t, "init", "--bare", f.bare)
	f.git(t, "remote", "add", "origin", f.bare)
	branch := strings.TrimSpace(f.git(t, "branch", "--show-current"))
	f.git(t, "push", "-q", "-u", "origin", branch)
	return branch
}

func (f *fixture) open(t *testing.T) *app {
	t.Helper()
	return startApp(t, f.repo, f.home, f.config)
}

func (f *fixture) read(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.repo, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

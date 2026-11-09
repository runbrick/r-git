package ops_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/ops"
)

type mockProvider struct {
	username string
	password string
	called   int
}

func (m *mockProvider) GetCredentials(url string) (string, string, error) {
	m.called++
	return m.username, m.password, nil
}

func TestAuthManager(t *testing.T) {
	if os.Getenv("R_GIT_TEST_ASKPASS") == "1" {
		ops.HandleAskpass()
		return
	}

	provider := &mockProvider{
		username: "testuser",
		password: "testpassword",
	}

	am, err := ops.NewAuthManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	defer am.Close()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	env := os.Environ()
	env = append(env, "R_GIT_TEST_ASKPASS=1")
	for k, v := range am.Env() {
		env = append(env, k+"="+v)
	}

	for i, e := range env {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			env[i] = "GIT_ASKPASS=" + executable
		}
	}

	cmd := exec.Command(executable, "Username for 'https://example.com':")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Askpass username failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "testuser" {
		t.Errorf("Expected testuser, got: %s", string(out))
	}

	cmd = exec.Command(executable, "Password for 'https://testuser@example.com':")
	cmd.Env = env
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("Askpass password failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "testpassword" {
		t.Errorf("Expected testpassword, got: %s", string(out))
	}

	if provider.called != 2 {
		t.Errorf("Expected provider to be called twice, got: %d", provider.called)
	}
}

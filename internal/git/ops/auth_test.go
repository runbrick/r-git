package ops_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/git/ops"
)

type mockProvider struct {
	username string
	password string
	called   int
}

func (m *mockProvider) Credentials(ctx context.Context, req ops.CredentialRequest) (ops.Credential, error) {
	m.called++
	return ops.Credential{Username: m.username, Password: m.password}, nil
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
	amEnv, err := am.Env()
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range amEnv {
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

	cmd = exec.Command(executable, "Password for 'https://example.com':")
	cmd.Env = env
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("Askpass password failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "testpassword" {
		t.Errorf("Expected testpassword, got: %s", string(out))
	}

	if provider.called != 1 {
		t.Errorf("Expected provider to be called once, got: %d", provider.called)
	}
}

type errorProvider struct{}

func (e *errorProvider) Credentials(ctx context.Context, req ops.CredentialRequest) (ops.Credential, error) {
	return ops.Credential{}, os.ErrNotExist
}

func TestAuthManager_Failures(t *testing.T) {
	if os.Getenv("R_GIT_TEST_ASKPASS") == "1" {
		ops.HandleAskpass()
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	// 1. Invalid token
	am, err := ops.NewAuthManager(&mockProvider{})
	if err != nil {
		t.Fatal(err)
	}
	defer am.Close()

	env := os.Environ()
	env = append(env, "R_GIT_TEST_ASKPASS=1")
	amEnv, _ := am.Env()
	for k, v := range amEnv {
		if k == "R_GIT_ASKPASS_TOKEN" {
			v = "invalidtoken"
		}
		env = append(env, k+"="+v)
	}
	for i, e := range env {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			env[i] = "GIT_ASKPASS=" + executable
		}
	}

	cmd := exec.Command(executable, "Username for 'https://example.com':")
	cmd.Env = env
	if err := cmd.Run(); err == nil {
		t.Error("Expected error for invalid token, got success")
	}

	// 2. Provider error
	amErr, _ := ops.NewAuthManager(&errorProvider{})
	defer amErr.Close()

	env2 := os.Environ()
	env2 = append(env2, "R_GIT_TEST_ASKPASS=1")
	amEnv2, _ := amErr.Env()
	for k, v := range amEnv2 {
		env2 = append(env2, k+"="+v)
	}
	for i, e := range env2 {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			env2[i] = "GIT_ASKPASS=" + executable
		}
	}

	cmd = exec.Command(executable, "Username for 'https://example.com':")
	cmd.Env = env2
	if err := cmd.Run(); err == nil {
		t.Error("Expected error for provider failure, got success")
	}
}

type slowProvider struct{}

func (s *slowProvider) Credentials(ctx context.Context, req ops.CredentialRequest) (ops.Credential, error) {
	<-ctx.Done()
	return ops.Credential{}, ctx.Err()
}

func TestAuthManager_CloseWithInflight(t *testing.T) {
	if os.Getenv("R_GIT_TEST_ASKPASS") == "1" {
		ops.HandleAskpass()
		return
	}

	am, err := ops.NewAuthManager(&slowProvider{})
	if err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	env := os.Environ()
	env = append(env, "R_GIT_TEST_ASKPASS=1")
	amEnv, _ := am.Env()
	for k, v := range amEnv {
		env = append(env, k+"="+v)
	}
	for i, e := range env {
		if strings.HasPrefix(e, "GIT_ASKPASS=") {
			env[i] = "GIT_ASKPASS=" + executable
		}
	}

	cmd := exec.Command(executable, "Username for 'https://example.com':")
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cmd.Process.Kill()
	}()

	err = am.Close()
	if err != nil {
		t.Fatal(err)
	}
}

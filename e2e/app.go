//go:build e2e

// Package e2e drives the real r-git binary through Viva's headless line protocol.
package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const replyTimeout = 60 * time.Second

// app is one headless r-git process and the line protocol on its pipes.
type app struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan string
	errs   chan error
	stderr *bytes.Buffer
	repo   string
	home   string
}

func startApp(t *testing.T, repo, home, configDir string) *app {
	t.Helper()
	bin := buildBinary(t)
	cmd := exec.Command(bin, repo)
	cmd.Env = append(os.Environ(),
		"VIVA_HEADLESS=1",
		"HOME="+home,
		"USERPROFILE="+home,
		"R_GIT_CONFIG_DIR="+configDir,
		"GIT_CONFIG_GLOBAL="+filepath.Join(home, "gitconfig-global"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(home, "gitconfig-system"),
	)
	cmd.Dir = repo
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	a := &app{
		t: t, cmd: cmd, stdin: stdin,
		lines:  make(chan string, 256),
		errs:   make(chan error, 1),
		stderr: &stderr, repo: repo, home: home,
	}
	go a.readStdout(stdout)
	t.Cleanup(func() {
		_ = stdin.Close()
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			t.Logf("killed r-git; stderr:\n%s", stderr.String())
		}
		if t.Failed() && stderr.Len() > 0 {
			t.Logf("r-git stderr:\n%s", stderr.String())
		}
	})
	// Run prints "ok ready WxH" before it reads a command. Drain it so the
	// first real command is not answered by that greeting.
	if _, _, err := a.readReply(); err != nil {
		t.Fatalf("headless protocol did not start: %v\nstderr:\n%s", err, stderr.String())
	}
	if _, err := a.roundTrip("resize 1600 1000"); err != nil {
		t.Fatalf("resize: %v\nstderr:\n%s", err, stderr.String())
	}
	return a
}

func (a *app) readStdout(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		a.lines <- scanner.Text()
	}
	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	a.errs <- err
}

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "r-git-e2e-bin-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "r-git")
		if os.PathSeparator == '\\' {
			builtBin += ".exe"
		}
		cmd := exec.Command("go", "build", "-tags", "vivahead", "-o", builtBin, "./cmd/r-git")
		cmd.Dir = moduleRoot(t)
		cmd.Env = append(os.Environ(), "GOFLAGS=-count=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build -tags vivahead: %w\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return builtBin
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// command sends one protocol line and fails the test unless it ends in ok.
func (a *app) command(line string) []string {
	a.t.Helper()
	data, err := a.roundTrip(line)
	if err != nil {
		a.t.Fatalf("%s: %v\non screen:\n%s\ntree:\n%s", line, err, strings.Join(a.safeTexts(), "\n"), strings.Join(a.safeTree(), "\n"))
	}
	return data
}

func (a *app) roundTrip(line string) ([]string, error) {
	if _, err := io.WriteString(a.stdin, line+"\n"); err != nil {
		return nil, err
	}
	data, _, err := a.readReply()
	if err != nil {
		return data, err
	}
	return data, nil
}

func (a *app) readReply() ([]string, string, error) {
	deadline := time.Now().Add(replyTimeout)
	var data []string
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return data, "", fmt.Errorf("timed out waiting for ok")
		}
		select {
		case got, ok := <-a.lines:
			if !ok {
				return data, "", io.EOF
			}
			if got == "ok" || strings.HasPrefix(got, "ok ") || strings.HasPrefix(got, "error") {
				if strings.HasPrefix(got, "error") {
					return data, got, fmt.Errorf("%s", got)
				}
				return data, got, nil
			}
			data = append(data, got)
		case err := <-a.errs:
			return data, "", err
		case <-time.After(remaining):
			return data, "", fmt.Errorf("timed out waiting for ok")
		}
	}
}

func (a *app) safeTree() []string {
	data, err := a.roundTrip("tree")
	if err != nil {
		return []string{"(tree failed: " + err.Error() + ")"}
	}
	return data
}

func (a *app) safeTexts() []string {
	data, err := a.roundTrip("texts")
	if err != nil {
		return []string{"(texts failed: " + err.Error() + ")", a.stderr.String()}
	}
	return data
}

func (a *app) click(label string) {
	a.t.Helper()
	a.command("click " + quote(label))
}

func (a *app) typeText(text string) {
	a.t.Helper()
	a.command("type " + text)
}

func (a *app) key(spec string) {
	a.t.Helper()
	a.command("key " + spec)
}

// waitFor blocks in the app until the painted texts contain needle.
// Opening a repository reads git before the first paint of the log, and
// that read takes longer than five seconds on a cold CI runner. The match is
// a substring, so a needle that is part of a label already on screen returns
// immediately.
func (a *app) waitFor(needle string) {
	a.t.Helper()
	a.command("waitfor " + quote(needle) + " 45000")
}

// texts returns every painted text line, optionally filtered.
func (a *app) texts(filter string) []string {
	a.t.Helper()
	line := "texts"
	if filter != "" {
		line += " " + quote(filter)
	}
	return a.command(line)
}

func (a *app) contains(needle string) bool {
	a.t.Helper()
	for _, line := range a.texts("") {
		if strings.Contains(unquoteText(line), needle) {
			return true
		}
	}
	return false
}

func (a *app) textDump() string {
	return strings.Join(a.texts(""), "\n")
}

// unquoteText pulls the quoted string out of a texts line.
func unquoteText(line string) string {
	start := strings.IndexByte(line, '"')
	if start < 0 {
		return line
	}
	value, err := strconv.Unquote(line[start:])
	if err != nil {
		return line
	}
	return value
}

// clickWhenEnabled waits for asynchronous loading to enable a named control,
// then takes the same label path a user takes. Labels are unique in the shell.
func (a *app) clickWhenEnabled(label string) {
	a.t.Helper()
	a.command("expect state " + quote(label) + " enabled reachable within 45000")
	a.click(label)
}

func quote(value string) string {
	if !strings.ContainsAny(value, " \t\"\\") {
		return value
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// Package command runs Git commands for the rest of the application.
package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Executor is the small command boundary consumed by higher-level packages.
type Executor interface {
	Run(context.Context, Cmd) (Result, error)
	Stream(context.Context, Cmd, func(Line)) (Result, error)
}

// Runner owns the executable, detected version, and clean base environment.
type Runner struct {
	Executable string
	Version    Version
	baseEnv    []string
	locksMu    sync.Mutex
	writeLocks map[string]*sync.Mutex
}

// Cmd describes one Git invocation. Config entries are passed as repeated -c values.
type Cmd struct {
	Dir             string
	Subcommand      string
	Args            []string
	Config          []string
	Env             map[string]string
	Stdin           []byte
	NoPager         bool
	Write           bool
	ReadOnly        bool
	NoOptionalLocks bool
}

// Result contains output separated by stream, retaining exact bytes and parsed lines.
type Result struct {
	Stdout      []byte
	Stderr      []byte
	OutputLines []string
	ErrorLines  []string
	ExitCode    int
}

// Line identifies the source stream for a complete line. CR-only separators are progress lines.
type Line struct {
	Text   string
	Stream StreamType
	CR     bool
}

type StreamType uint8

const (
	Stdout StreamType = iota
	Stderr
)

// ExitError represents a Git process that started and returned a non-zero exit code.
type ExitError struct {
	Code    int
	Command string
	Stderr  string
}

func (e *ExitError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("git command exited with code %d: %s", e.Code, e.Command)
	}
	return fmt.Sprintf("git command exited with code %d: %s: %s", e.Code, e.Command, strings.TrimSpace(e.Stderr))
}

// New detects git on PATH, reads its version, and enforces the supported minimum.
func New() (*Runner, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("find git executable: %w", err)
	}
	return NewWithExecutable(path)
}

// NewWithExecutable validates an explicit Git executable.
func NewWithExecutable(path string) (*Runner, error) {
	cmd := exec.Command(path, "version")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("identify git executable %q: %w", path, err)
	}
	v, err := ParseVersion(string(out))
	if err != nil {
		return nil, err
	}
	if !v.Supported() {
		return nil, fmt.Errorf("git %s is too old; minimum supported version is %s", v, MinimumVersion())
	}
	return &Runner{Executable: path, Version: v, baseEnv: cleanEnvironment(os.Environ()), writeLocks: map[string]*sync.Mutex{}}, nil
}

// NewForTesting constructs a runner with a provided environment and version.
func NewForTesting(path string, version Version, env []string) *Runner {
	return &Runner{Executable: path, Version: version, baseEnv: cleanEnvironment(env), writeLocks: map[string]*sync.Mutex{}}
}

func cleanEnvironment(env []string) []string {
	blocked := map[string]bool{"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_COMMON_DIR": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_NAMESPACE": true}
	result := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || blocked[strings.ToUpper(key)] {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func (r *Runner) environment(extra map[string]string) []string {
	values := make(map[string]string, len(r.baseEnv)+len(extra)+3)
	for _, entry := range r.baseEnv {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			values[k] = v
		}
	}
	for key, value := range extra {
		values[key] = value
	}
	values["LC_ALL"] = "C"
	values["LANGUAGE"] = ""
	values["GIT_TERMINAL_PROMPT"] = "0"
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func (r *Runner) lock(c Cmd) func() {
	write := !c.ReadOnly && (c.Write || isWriteCommand(c.Subcommand, c.Args))
	if !write {
		return func() {}
	}
	root := c.Dir
	if abs, err := filepath.Abs(root); err == nil {
		root = repositoryRoot(filepath.Clean(abs))
	}
	r.locksMu.Lock()
	lock := r.writeLocks[root]
	if lock == nil {
		lock = &sync.Mutex{}
		r.writeLocks[root] = lock
	}
	r.locksMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func isWriteCommand(command string, args []string) bool {
	switch command {
	case "add", "checkout", "sparse-checkout", "commit", "commit-tree", "cherry-pick", "init", "merge", "mv", "pull", "rebase", "reset", "restore", "revert", "rm", "update-index", "update-ref", "hash-object", "hook":
		return true
	case "stash":
		return len(args) == 0 || (args[0] != "list" && args[0] != "show")
	case "submodule":
		return len(args) > 0 && args[0] != "status" && args[0] != "summary" && args[0] != "foreach"
	default:
		return false
	}
}

func (r *Runner) Run(ctx context.Context, c Cmd) (Result, error) { return r.execute(ctx, c, nil) }

// Stream emits complete stdout and stderr lines while retaining the captured result.
func (r *Runner) Stream(ctx context.Context, c Cmd, onLine func(Line)) (Result, error) {
	return r.execute(ctx, c, onLine)
}

func (r *Runner) execute(ctx context.Context, c Cmd, onLine func(Line)) (Result, error) {
	unlock := r.lock(c)
	defer unlock()
	args := r.argv(c)
	cmd := exec.CommandContext(ctx, r.Executable, args...)
	cmd.Dir, cmd.Env = c.Dir, r.environment(c.Env)
	cmd.Stdin = bytes.NewReader(c.Stdin)
	configureProcess(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, err
	}
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	var result Result
	var mu sync.Mutex
	var callbackMu sync.Mutex
	if onLine != nil {
		callback := onLine
		onLine = func(line Line) { callbackMu.Lock(); defer callbackMu.Unlock(); callback(line) }
	}
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); r.readStream(stdout, Stdout, onLine, &result, &mu) }()
	go func() { defer readers.Done(); r.readStream(stderr, Stderr, onLine, &result, &mu) }()
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	readers.Wait()
	waitErr := <-wait
	result.ExitCode = 0
	if waitErr != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, &ExitError{Code: result.ExitCode, Command: displayCommand(r.Executable, args), Stderr: string(result.Stderr)}
		}
		return result, waitErr
	}
	return result, nil
}

func repositoryRoot(dir string) string {
	for current := dir; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return dir
		}
	}
}

func (r *Runner) readStream(reader io.Reader, stream StreamType, callback func(Line), result *Result, mu *sync.Mutex) {
	var b bytes.Buffer
	buf := make([]byte, 8192)
	splitter := NewTextSplitter(func(text string, cr bool) {
		line := Line{Text: text, Stream: stream, CR: cr}
		mu.Lock()
		if stream == Stdout {
			result.OutputLines = append(result.OutputLines, text)
		} else {
			result.ErrorLines = append(result.ErrorLines, text)
		}
		mu.Unlock()
		if callback != nil {
			callback(line)
		}
	})
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
			splitter.Process(buf[:n])
		}
		if err != nil {
			break
		}
	}
	splitter.Flush()
	mu.Lock()
	if stream == Stdout {
		result.Stdout = append(result.Stdout, b.Bytes()...)
	} else {
		result.Stderr = append(result.Stderr, b.Bytes()...)
	}
	mu.Unlock()
}

func (r *Runner) argv(c Cmd) []string {
	args := make([]string, 0, 6+2*len(c.Config)+len(c.Args))
	args = append(args, "-c", "core.quotepath=false", "-c", "log.showSignature=false")
	for _, config := range c.Config {
		args = append(args, "-c", config)
	}
	if c.NoPager {
		args = append(args, "--no-pager")
	}
	if c.NoOptionalLocks {
		args = append(args, "--no-optional-locks")
	}
	args = append(args, c.Subcommand)
	return append(args, c.Args...)
}

func displayCommand(executable string, args []string) string {
	return strings.Join(append([]string{executable}, args...), " ")
}

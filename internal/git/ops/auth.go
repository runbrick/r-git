package ops

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/runbrick/r-git/internal/git/command"
)

type CredentialKind int

const (
	CredentialKindUnknown CredentialKind = iota
	CredentialKindUsername
	CredentialKindPassword
)

type CredentialRequest struct {
	Prompt string
	URL    string
	Kind   CredentialKind
}

type Credential struct {
	Username string
	Password string
}

// CredentialProvider is the interface for providing git credentials.
type CredentialProvider interface {
	Credentials(ctx context.Context, req CredentialRequest) (Credential, error)
}

// AuthManager sets up environment variables to intercept git credentials requests.
// It acts as a local askpass server via loopback TCP.
// Threat model: Any local process can connect to the port, so it is protected by a 256-bit token
// passed in the environment to the git process.
type AuthManager struct {
	provider CredentialProvider
	token    string
	listener net.Listener
	mu       sync.Mutex
	cache    map[string]Credential
	done     chan struct{}
	wg       sync.WaitGroup
}

func NewAuthManager(provider CredentialProvider) (*AuthManager, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	am := &AuthManager{
		provider: provider,
		token:    token,
		listener: listener,
		cache:    make(map[string]Credential),
		done:     make(chan struct{}),
	}

	am.wg.Add(1)
	go am.serve()

	return am, nil
}

func (am *AuthManager) Close() error {
	close(am.done)
	err := am.listener.Close()
	am.wg.Wait()
	return err
}

func (am *AuthManager) Env() (map[string]string, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}

	port := am.listener.Addr().(*net.TCPAddr).Port

	return map[string]string{
		"GIT_ASKPASS":         executable,
		"R_GIT_ASKPASS_PORT":  fmt.Sprintf("%d", port),
		"R_GIT_ASKPASS_TOKEN": am.token,
	}, nil
}

func (am *AuthManager) serve() {
	defer am.wg.Done()
	for {
		conn, err := am.listener.Accept()
		if err != nil {
			select {
			case <-am.done:
				return
			default:
				// Temporary error or listener closed
				continue
			}
		}
		am.wg.Add(1)
		go am.handleConnection(conn)
	}
}

var urlPattern = regexp.MustCompile(`for '([^']+)'`)

func parsePrompt(prompt string) CredentialRequest {
	req := CredentialRequest{
		Prompt: prompt,
		Kind:   CredentialKindUnknown,
	}

	lower := strings.ToLower(prompt)
	if strings.Contains(lower, "username") {
		req.Kind = CredentialKindUsername
	} else if strings.Contains(lower, "password") || strings.Contains(lower, "passphrase") {
		req.Kind = CredentialKindPassword
	}

	matches := urlPattern.FindStringSubmatch(prompt)
	if len(matches) > 1 {
		req.URL = matches[1]
	}

	return req
}

func (am *AuthManager) handleConnection(conn net.Conn) {
	defer am.wg.Done()
	defer conn.Close()

	// 5 seconds to read request
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)

	if !scanner.Scan() {
		fmt.Fprintln(conn, "ERROR: token missing")
		return
	}
	clientToken := scanner.Text()

	if subtle.ConstantTimeCompare([]byte(clientToken), []byte(am.token)) != 1 {
		fmt.Fprintln(conn, "ERROR: invalid token")
		return
	}

	if !scanner.Scan() {
		fmt.Fprintln(conn, "ERROR: prompt missing")
		return
	}
	prompt := scanner.Text()

	req := parsePrompt(prompt)

	// We'll give 2 minutes for the user to answer the prompt
	conn.SetDeadline(time.Now().Add(2 * time.Minute))

	am.mu.Lock()
	cred, cached := am.cache[req.URL]
	am.mu.Unlock()

	if !cached || req.URL == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// If context ends up canceled due to connection closed, handle that via a goroutine
		go func() {
			select {
			case <-am.done:
				cancel()
			case <-ctx.Done():
			}
		}()

		var err error
		cred, err = am.provider.Credentials(ctx, req)
		if err != nil {
			fmt.Fprintln(conn, "ERROR: provider error")
			return
		}
		if req.URL != "" {
			am.mu.Lock()
			am.cache[req.URL] = cred
			am.mu.Unlock()
		}
	}

	fmt.Fprintln(conn, "OK")
	switch req.Kind {
	case CredentialKindUsername:
		fmt.Fprintln(conn, cred.Username)
	case CredentialKindPassword:
		fmt.Fprintln(conn, cred.Password)
	default:
		fmt.Fprintln(conn, cred.Password)
	}
}

// HandleAskpass handles the askpass execution if invoked as GIT_ASKPASS.
// It connects back to the AuthManager, sends the prompt, and prints the result.
func HandleAskpass() bool {
	port := os.Getenv("R_GIT_ASKPASS_PORT")
	token := os.Getenv("R_GIT_ASKPASS_TOKEN")
	if port == "" || token == "" {
		return false
	}

	prompt := ""
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 2*time.Second)
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(3 * time.Minute))

	fmt.Fprintln(conn, token)
	fmt.Fprintln(conn, prompt)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		os.Exit(1)
	}

	status := scanner.Text()
	if status != "OK" {
		os.Exit(1)
	}

	if scanner.Scan() {
		fmt.Println(scanner.Text())
		os.Exit(0)
	}

	os.Exit(1)
	return true
}

func ExecuteWithAuth(ctx context.Context, runner *command.Runner, cmd command.Cmd, provider CredentialProvider) (*command.Result, error) {
	if provider != nil {
		am, err := NewAuthManager(provider)
		if err != nil {
			return nil, err
		}
		defer am.Close()

		env, err := am.Env()
		if err != nil {
			return nil, err
		}

		if cmd.Env == nil {
			cmd.Env = make(map[string]string)
		}
		for k, v := range env {
			cmd.Env[k] = v
		}
	}

	res, err := runner.Run(ctx, cmd)
	return &res, err
}

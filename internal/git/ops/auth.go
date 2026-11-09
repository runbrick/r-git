package ops

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
)

// CredentialProvider is the interface for providing git credentials.
type CredentialProvider interface {
	// GetCredentials returns the username and password for a given URL.
	// It is called when git prompts for credentials.
	GetCredentials(url string) (username string, password string, err error)
}

// AuthManager sets up environment variables to intercept git credentials requests.
// It acts as a local askpass server.
type AuthManager struct {
	provider CredentialProvider
	token    string
	listener net.Listener
}

func NewAuthManager(provider CredentialProvider) (*AuthManager, error) {
	tokenBytes := make([]byte, 16)
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
	}

	go am.serve()

	return am, nil
}

func (am *AuthManager) Close() error {
	return am.listener.Close()
}

func (am *AuthManager) Env() map[string]string {
	executable, err := os.Executable()
	if err != nil {
		return nil
	}

	port := am.listener.Addr().(*net.TCPAddr).Port

	return map[string]string{
		"GIT_ASKPASS":         executable,
		"R_GIT_ASKPASS_PORT":  fmt.Sprintf("%d", port),
		"R_GIT_ASKPASS_TOKEN": am.token,
	}
}

// serve handles incoming askpass requests from child processes.
func (am *AuthManager) serve() {
	for {
		conn, err := am.listener.Accept()
		if err != nil {
			return
		}
		go am.handleConnection(conn)
	}
}

func (am *AuthManager) handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	// Read token
	if !scanner.Scan() {
		return
	}
	if scanner.Text() != am.token {
		return
	}

	// Read prompt
	if !scanner.Scan() {
		return
	}
	prompt := scanner.Text()

	username, password, err := am.provider.GetCredentials(prompt)
	if err != nil {
		return
	}

	if strings.Contains(strings.ToLower(prompt), "username") {
		fmt.Fprintln(conn, username)
	} else if strings.Contains(strings.ToLower(prompt), "password") {
		fmt.Fprintln(conn, password)
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

	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintln(conn, token)
	fmt.Fprintln(conn, prompt)

	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	os.Exit(0)
	return true
}

func ExecuteWithAuth(ctx context.Context, runner *command.Runner, cmd command.Cmd, provider CredentialProvider) (*command.Result, error) {
	if provider != nil {
		am, err := NewAuthManager(provider)
		if err != nil {
			return nil, err
		}
		defer am.Close()

		if cmd.Env == nil {
			cmd.Env = make(map[string]string)
		}
		for k, v := range am.Env() {
			cmd.Env[k] = v
		}
	}

	res, err := runner.Run(ctx, cmd)
	return &res, err
}

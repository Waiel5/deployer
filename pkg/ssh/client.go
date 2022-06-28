package ssh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/Waiel5/deployer/pkg/config"
)

// Client wraps an SSH connection and provides command execution.
type Client struct {
	conn *ssh.Client
	host string
}

// Execute runs a command on the remote host and returns the combined output.
func (c *Client) Execute(command string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("creating session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(command); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = stdout.String()
		}
		return "", fmt.Errorf("command failed: %s: %s", err, strings.TrimSpace(errMsg))
	}

	return stdout.String(), nil
}

// ExecuteQuiet runs a command and returns only the error, discarding output.
func (c *Client) ExecuteQuiet(command string) error {
	_, err := c.Execute(command)
	return err
}

// Host returns the host address this client is connected to.
func (c *Client) Host() string {
	return c.host
}

// Close closes the underlying SSH connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ConnectionPool manages reusable SSH connections to multiple hosts.
type ConnectionPool struct {
	mu      sync.Mutex
	clients map[string]*Client
}

// NewConnectionPool creates a new empty connection pool.
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		clients: make(map[string]*Client),
	}
}

// Get retrieves or creates an SSH connection to the given host.
func (p *ConnectionPool) Get(host string, cfg config.SSHConfig) (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if client, ok := p.clients[host]; ok {
		return client, nil
	}

	client, err := connect(host, cfg)
	if err != nil {
		return nil, err
	}

	p.clients[host] = client
	return client, nil
}

// CloseAll closes all connections in the pool.
func (p *ConnectionPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for host, client := range p.clients {
		client.Close()
		delete(p.clients, host)
	}
}

// connect establishes a new SSH connection to the given host.
func connect(host string, cfg config.SSHConfig) (*Client, error) {
	authMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, fmt.Errorf("building auth methods: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, cfg.Port)
	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}

	return &Client{conn: conn, host: host}, nil
}

// buildAuthMethods creates SSH authentication methods from config.
func buildAuthMethods(cfg config.SSHConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if cfg.KeyPath != "" {
		keyPath := expandPath(cfg.KeyPath)
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("reading SSH key %s: %w", keyPath, err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parsing SSH key: %w", err)
		}

		methods = append(methods, ssh.PublicKeys(signer))
	}

	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no authentication methods configured")
	}

	return methods, nil
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

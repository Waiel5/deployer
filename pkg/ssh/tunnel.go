package ssh

import (
	"fmt"
	"io"
	"net"
	"sync"
)

// Tunnel represents an SSH tunnel for forwarding local connections to remote hosts.
type Tunnel struct {
	client     *Client
	localAddr  string
	remoteAddr string
	listener   net.Listener
	done       chan struct{}
	mu         sync.Mutex
	active     bool
}

// NewTunnel creates a new SSH tunnel configuration.
func NewTunnel(client *Client, localPort int, remoteHost string, remotePort int) *Tunnel {
	return &Tunnel{
		client:     client,
		localAddr:  fmt.Sprintf("127.0.0.1:%d", localPort),
		remoteAddr: fmt.Sprintf("%s:%d", remoteHost, remotePort),
		done:       make(chan struct{}),
	}
}

// Start begins listening and forwarding connections through the SSH tunnel.
func (t *Tunnel) Start() error {
	var err error
	t.listener, err = net.Listen("tcp", t.localAddr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", t.localAddr, err)
	}

	t.mu.Lock()
	t.active = true
	t.mu.Unlock()

	go t.acceptLoop()
	return nil
}

// Stop closes the tunnel and stops accepting connections.
func (t *Tunnel) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.active {
		return
	}

	t.active = false
	close(t.done)
	if t.listener != nil {
		t.listener.Close()
	}
}

// LocalAddr returns the local address the tunnel is listening on.
func (t *Tunnel) LocalAddr() string {
	if t.listener != nil {
		return t.listener.Addr().String()
	}
	return t.localAddr
}

func (t *Tunnel) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.done:
				return
			default:
				continue
			}
		}
		go t.forward(conn)
	}
}

func (t *Tunnel) forward(local net.Conn) {
	remote, err := t.client.conn.Dial("tcp", t.remoteAddr)
	if err != nil {
		local.Close()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Local -> Remote
	go func() {
		defer wg.Done()
		io.Copy(remote, local)
		remote.Close()
	}()

	// Remote -> Local
	go func() {
		defer wg.Done()
		io.Copy(local, remote)
		local.Close()
	}()

	wg.Wait()
}

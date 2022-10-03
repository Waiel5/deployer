package docker

import (
	"fmt"
	"strings"

	"github.com/Waiel5/deployer/pkg/ssh"
)

// Client executes Docker commands on a remote host via SSH.
type Client struct {
	ssh *ssh.Client
}

// ContainerInfo holds information about a running container.
type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	Status  string
	Running bool
	Uptime  string
	Ports   []string
}

// RunOptions specifies container run parameters.
type RunOptions struct {
	Name        string
	Image       string
	Ports       []string
	Env         map[string]string
	Volumes     []string
	NetworkMode string
	Restart     string
	Command     string
}

// NewClient creates a new Docker client that operates over SSH.
func NewClient(sshClient *ssh.Client) *Client {
	return &Client{ssh: sshClient}
}

// Pull fetches a Docker image on the remote host.
func (c *Client) Pull(image string) error {
	cmd := fmt.Sprintf("docker pull %s", image)
	_, err := c.ssh.Execute(cmd)
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", image, err)
	}
	return nil
}

// Run starts a new container with the given options.
func (c *Client) Run(opts RunOptions) error {
	args := []string{"docker run -d"}

	if opts.Name != "" {
		// Remove existing container with the same name first
		_ = c.Remove(opts.Name)
		args = append(args, fmt.Sprintf("--name %s", opts.Name))
	}

	for _, port := range opts.Ports {
		args = append(args, fmt.Sprintf("-p %s", port))
	}

	for key, val := range opts.Env {
		args = append(args, fmt.Sprintf("-e %s=%s", key, shellEscape(val)))
	}

	for _, vol := range opts.Volumes {
		args = append(args, fmt.Sprintf("-v %s", vol))
	}

	if opts.NetworkMode != "" {
		args = append(args, fmt.Sprintf("--network %s", opts.NetworkMode))
	}

	if opts.Restart != "" {
		args = append(args, fmt.Sprintf("--restart %s", opts.Restart))
	}

	args = append(args, opts.Image)

	if opts.Command != "" {
		args = append(args, opts.Command)
	}

	cmd := strings.Join(args, " ")
	_, err := c.ssh.Execute(cmd)
	if err != nil {
		return fmt.Errorf("running container: %w", err)
	}
	return nil
}

// Stop stops a running container with the given timeout in seconds.
func (c *Client) Stop(name string, timeout int) error {
	cmd := fmt.Sprintf("docker stop -t %d %s", timeout, name)
	return c.ssh.ExecuteQuiet(cmd)
}

// Remove removes a container (forced).
func (c *Client) Remove(name string) error {
	cmd := fmt.Sprintf("docker rm -f %s 2>/dev/null || true", name)
	return c.ssh.ExecuteQuiet(cmd)
}

// Inspect retrieves information about a container.
func (c *Client) Inspect(name string) (*ContainerInfo, error) {
	// Get container status using docker inspect with format
	format := `{{.Id}}|{{.Config.Image}}|{{.State.Status}}|{{.State.Running}}|{{.State.StartedAt}}`
	cmd := fmt.Sprintf("docker inspect --format '%s' %s", format, name)

	output, err := c.ssh.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("inspecting container %s: %w", name, err)
	}

	parts := strings.SplitN(strings.TrimSpace(output), "|", 5)
	if len(parts) < 5 {
		return nil, fmt.Errorf("unexpected inspect output: %s", output)
	}

	info := &ContainerInfo{
		ID:      parts[0][:12],
		Name:    name,
		Image:   parts[1],
		Status:  parts[2],
		Running: parts[3] == "true",
		Uptime:  formatUptime(parts[4]),
	}

	// Get port mappings
	portCmd := fmt.Sprintf("docker port %s 2>/dev/null || true", name)
	portOutput, _ := c.ssh.Execute(portCmd)
	if portOutput != "" {
		for _, line := range strings.Split(strings.TrimSpace(portOutput), "\n") {
			if line != "" {
				info.Ports = append(info.Ports, line)
			}
		}
	}

	return info, nil
}

// ListContainers returns all containers matching the project prefix.
func (c *Client) ListContainers(prefix string) ([]ContainerInfo, error) {
	cmd := fmt.Sprintf("docker ps -a --filter name=%s --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.Ports}}'", prefix)
	output, err := c.ssh.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	var containers []ContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}

		running := strings.HasPrefix(parts[3], "Up")
		containers = append(containers, ContainerInfo{
			ID:      parts[0],
			Name:    parts[1],
			Image:   parts[2],
			Status:  parts[3],
			Running: running,
			Ports:   strings.Split(parts[4], ","),
		})
	}

	return containers, nil
}

// Logs returns container logs.
func (c *Client) Logs(name string, tail int, follow bool) (string, error) {
	cmd := fmt.Sprintf("docker logs --tail %d", tail)
	if follow {
		cmd += " -f"
	}
	cmd += " " + name

	return c.ssh.Execute(cmd)
}

// formatUptime converts a Docker timestamp to a human-readable uptime string.
func formatUptime(startedAt string) string {
	if startedAt == "" || startedAt == "0001-01-01T00:00:00Z" {
		return "not started"
	}
	// Simplified: just return the raw timestamp for now
	// In a real implementation, we'd calculate the duration
	if len(startedAt) > 19 {
		return startedAt[:19]
	}
	return startedAt
}

// shellEscape wraps a value in single quotes for safe shell usage.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

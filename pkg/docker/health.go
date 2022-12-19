package docker

import (
	"fmt"
	"strings"
	"time"

	"github.com/Waiel5/deployer/pkg/config"
	"github.com/Waiel5/deployer/pkg/ssh"
)

// HealthChecker polls a container's health status.
type HealthChecker struct {
	client *ssh.Client
	config config.HealthCheckConfig
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(client *ssh.Client, cfg config.HealthCheckConfig) *HealthChecker {
	return &HealthChecker{
		client: client,
		config: cfg,
	}
}

// Wait polls the health check endpoint until it passes or retries are exhausted.
func (h *HealthChecker) Wait() error {
	interval := h.config.GetInterval()
	timeout := h.config.GetTimeout()
	retries := h.config.Retries

	if retries <= 0 {
		retries = 3
	}

	// Initial delay to let the container start
	time.Sleep(2 * time.Second)

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		err := h.check(timeout)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt < retries {
			time.Sleep(interval)
		}
	}

	return fmt.Errorf("health check failed after %d attempts: %w", retries, lastErr)
}

// check performs a single health check based on the configured type.
func (h *HealthChecker) check(timeout time.Duration) error {
	switch h.config.Type {
	case "http":
		return h.checkHTTP(timeout)
	case "tcp":
		return h.checkTCP(timeout)
	case "command":
		return h.checkCommand()
	default:
		return fmt.Errorf("unknown health check type: %s", h.config.Type)
	}
}

// checkHTTP performs an HTTP health check via curl on the remote host.
func (h *HealthChecker) checkHTTP(timeout time.Duration) error {
	endpoint := h.config.Endpoint
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", h.config.Port, endpoint)
	cmd := fmt.Sprintf(
		"curl -sf -o /dev/null -w '%%{http_code}' --max-time %d %s",
		int(timeout.Seconds()), url,
	)

	output, err := h.client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("HTTP health check failed: %w", err)
	}

	statusCode := strings.TrimSpace(output)
	if statusCode != "200" && statusCode != "201" && statusCode != "204" {
		return fmt.Errorf("HTTP health check returned status %s", statusCode)
	}

	return nil
}

// checkTCP attempts a TCP connection to the specified port on the remote host.
func (h *HealthChecker) checkTCP(timeout time.Duration) error {
	cmd := fmt.Sprintf(
		"timeout %d bash -c '</dev/tcp/127.0.0.1/%d' 2>/dev/null",
		int(timeout.Seconds()), h.config.Port,
	)

	_, err := h.client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("TCP health check failed on port %d: %w", h.config.Port, err)
	}

	return nil
}

// checkCommand runs a custom command and checks the exit code.
func (h *HealthChecker) checkCommand() error {
	_, err := h.client.Execute(h.config.Command)
	if err != nil {
		return fmt.Errorf("command health check failed: %w", err)
	}
	return nil
}

// CheckResult represents the result of a health check.
type CheckResult struct {
	Healthy  bool
	Message  string
	Duration time.Duration
	Attempt  int
}

// RunDetailed performs health checks and returns detailed results for each attempt.
func (h *HealthChecker) RunDetailed() []CheckResult {
	interval := h.config.GetInterval()
	timeout := h.config.GetTimeout()
	retries := h.config.Retries

	if retries <= 0 {
		retries = 3
	}

	time.Sleep(2 * time.Second)

	var results []CheckResult
	for attempt := 1; attempt <= retries; attempt++ {
		start := time.Now()
		err := h.check(timeout)
		duration := time.Since(start)

		result := CheckResult{
			Healthy:  err == nil,
			Duration: duration,
			Attempt:  attempt,
		}

		if err != nil {
			result.Message = err.Error()
		} else {
			result.Message = "healthy"
		}

		results = append(results, result)

		if err == nil {
			break
		}

		if attempt < retries {
			time.Sleep(interval)
		}
	}

	return results
}

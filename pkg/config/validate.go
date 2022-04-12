package config

import (
	"fmt"
	"net"
	"strings"
)

// ValidationError collects multiple validation errors.
type ValidationError struct {
	Errors []string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("validation failed:\n  - %s", strings.Join(v.Errors, "\n  - "))
}

func (v *ValidationError) add(msg string, args ...interface{}) {
	v.Errors = append(v.Errors, fmt.Sprintf(msg, args...))
}

func (v *ValidationError) hasErrors() bool {
	return len(v.Errors) > 0
}

// Validate checks the config for errors and returns a descriptive error.
func Validate(cfg *Config) error {
	v := &ValidationError{}

	if cfg.Project == "" {
		v.add("project name is required")
	}

	if cfg.SSH.User == "" {
		v.add("ssh.user is required")
	}

	if cfg.SSH.Port < 1 || cfg.SSH.Port > 65535 {
		v.add("ssh.port must be between 1 and 65535")
	}

	if cfg.SSH.KeyPath == "" && cfg.SSH.Password == "" {
		v.add("ssh.key_path or ssh.password is required")
	}

	if len(cfg.Services) == 0 {
		v.add("at least one service must be defined")
	}

	serviceNames := make(map[string]bool)
	for i, svc := range cfg.Services {
		prefix := fmt.Sprintf("services[%d]", i)

		if svc.Name == "" {
			v.add("%s: name is required", prefix)
		} else if serviceNames[svc.Name] {
			v.add("%s: duplicate service name %q", prefix, svc.Name)
		}
		serviceNames[svc.Name] = true

		if svc.Image == "" {
			v.add("%s (%s): image is required", prefix, svc.Name)
		}

		if len(svc.Hosts) == 0 {
			v.add("%s (%s): at least one host is required", prefix, svc.Name)
		}

		for j, host := range svc.Hosts {
			if net.ParseIP(host) == nil {
				// Not a plain IP, check if it looks like a valid hostname
				if !isValidHostname(host) {
					v.add("%s (%s): hosts[%d] %q is not a valid IP or hostname",
						prefix, svc.Name, j, host)
				}
			}
		}

		for j, port := range svc.Ports {
			if !isValidPortMapping(port) {
				v.add("%s (%s): ports[%d] %q is not a valid port mapping (use host:container)",
					prefix, svc.Name, j, port)
			}
		}

		strategy := svc.Deploy.Strategy
		if strategy != "rolling" && strategy != "blue-green" {
			v.add("%s (%s): deploy.strategy must be 'rolling' or 'blue-green', got %q",
				prefix, svc.Name, strategy)
		}

		if svc.HealthCheck.Enabled {
			hc := svc.HealthCheck
			switch hc.Type {
			case "http":
				if hc.Endpoint == "" {
					v.add("%s (%s): health_check.endpoint is required for http checks",
						prefix, svc.Name)
				}
				if hc.Port == 0 {
					v.add("%s (%s): health_check.port is required for http checks",
						prefix, svc.Name)
				}
			case "tcp":
				if hc.Port == 0 {
					v.add("%s (%s): health_check.port is required for tcp checks",
						prefix, svc.Name)
				}
			case "command":
				if hc.Command == "" {
					v.add("%s (%s): health_check.command is required for command checks",
						prefix, svc.Name)
				}
			default:
				v.add("%s (%s): health_check.type must be 'http', 'tcp', or 'command', got %q",
					prefix, svc.Name, hc.Type)
			}
		}
	}

	if v.hasErrors() {
		return v
	}
	return nil
}

func isValidHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	parts := strings.Split(host, ".")
	for _, part := range parts {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
		for i, c := range part {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || (c == '-' && i > 0)) {
				return false
			}
		}
	}
	return true
}

func isValidPortMapping(port string) bool {
	parts := strings.Split(port, ":")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

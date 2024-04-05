# deployer

[![CI](https://github.com/Waiel5/deployer/actions/workflows/ci.yml/badge.svg)](https://github.com/Waiel5/deployer/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Waiel5/deployer)](https://goreportcard.com/report/github.com/Waiel5/deployer)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Zero-downtime container deployments via SSH. Rolling updates, health checks, and instant rollback without Kubernetes.

## Why deployer?

You have a few servers. You want to deploy Docker containers to them with zero downtime. You don't need Kubernetes. You don't want to manage a Swarm cluster. You just want to SSH in, pull the new image, and swap containers -- with health checks and automatic rollback if something goes wrong.

**deployer** does exactly that.

| Feature | deployer | docker-compose | Kubernetes |
|---|---|---|---|
| Multi-host deployments | Yes | No (single host) | Yes |
| Rolling updates | Yes | No | Yes |
| Health checks + auto-rollback | Yes | No | Yes |
| Setup complexity | One binary + SSH | Docker installed | Cluster + etcd + API server |
| Learning curve | 10 minutes | 30 minutes | Weeks |
| Resource overhead | None | None | High |
| Config format | Single YAML | Single YAML | Many YAMLs |

## Install

```bash
# From source
go install github.com/Waiel5/deployer@latest

# Or download a binary from releases
curl -sL https://github.com/Waiel5/deployer/releases/latest/download/deployer-$(uname -s | tr '[:upper:]' '[:lower:]')-amd64 -o /usr/local/bin/deployer
chmod +x /usr/local/bin/deployer
```

## Quick start

```bash
# Scaffold a config file
deployer init

# Edit deployer.yml with your hosts and services
vim deployer.yml

# Deploy
deployer deploy

# Check status
deployer status

# View logs
deployer logs -s api

# Rollback if needed
deployer rollback -s api
```

## Configuration

```yaml
project: myapp

ssh:
  user: deploy
  port: 22
  key_path: ~/.ssh/id_rsa

services:
  - name: api
    image: ghcr.io/myorg/api:v1.2.0
    hosts:
      - 10.0.1.10
      - 10.0.1.11
    ports:
      - "80:3000"
    environment:
      NODE_ENV: production
      DATABASE_URL: postgres://db:5432/myapp
    volumes:
      - /data/api:/app/data
    deploy:
      strategy: rolling   # or blue-green
      batch_size: 1
      delay: 10s
    health_check:
      enabled: true
      type: http           # http, tcp, or command
      endpoint: /healthz
      port: 3000
      interval: 5s
      timeout: 10s
      retries: 5
```

## How it works

1. **Parse** `deployer.yml` for services, hosts, and deploy strategy
2. **SSH** into each host (connection pooling, key-based auth)
3. **Pull** the new Docker image
4. **Stop** the old container gracefully
5. **Start** the new container with the same config
6. **Health check** the new container (HTTP, TCP, or command)
7. **Rollback** automatically if the health check fails

For rolling updates, hosts are updated in batches with a configurable delay between them. If any host fails, deployment stops (unless `--force` is used).

## Commands

### `deployer deploy`

Deploy all services (or specific ones with `--service`).

```bash
deployer deploy                    # deploy everything
deployer deploy -s api,worker      # deploy specific services
deployer deploy --dry-run          # show plan without executing
deployer deploy --force            # continue even if a host fails
```

### `deployer rollback`

Rollback a service to its previous version. Version history is tracked in `.deployer/versions.json`.

```bash
deployer rollback -s api                     # rollback to previous version
deployer rollback -s api -t myorg/api:v1.1   # rollback to specific version
```

### `deployer status`

Show the current state of all services across all hosts.

```bash
deployer status
```

```
┌─────────┬────────────┬─────────┬──────────────────┬─────────────────────┬──────────┐
│ Service │ Host       │ Status  │ Image            │ Uptime              │ Ports    │
├─────────┼────────────┼─────────┼──────────────────┼─────────────────────┼──────────┤
│ api     │ 10.0.1.10  │ RUNNING │ myorg/api:v1.2.0 │ 2024-01-15T10:30:00 │ 80:3000  │
│ api     │ 10.0.1.11  │ RUNNING │ myorg/api:v1.2.0 │ 2024-01-15T10:31:00 │ 80:3000  │
│ worker  │ 10.0.1.20  │ RUNNING │ myorg/wrk:v1.2.0 │ 2024-01-15T10:32:00 │ -        │
└─────────┴────────────┴─────────┴──────────────────┴─────────────────────┴──────────┘
```

### `deployer logs`

Tail logs from remote containers.

```bash
deployer logs -s api               # last 100 lines from all hosts
deployer logs -s api -n 500        # last 500 lines
deployer logs -s api --host 10.0.1.10  # from a specific host
deployer logs -s api -f            # follow (stream)
```

### `deployer init`

Generate a starter `deployer.yml` in the current directory.

```bash
deployer init
deployer init --force   # overwrite existing
```

## Deploy strategies

### Rolling update (default)

Updates hosts one at a time (or in configurable batch sizes). Each batch waits for health checks before proceeding. If a health check fails, the failed host is rolled back and deployment stops.

```yaml
deploy:
  strategy: rolling
  batch_size: 1
  delay: 10s
```

### Blue-green

Splits hosts into two groups. Updates the first group, verifies health, then updates the second group. Provides a faster rollback path since half the hosts are always running the old version during deployment.

```yaml
deploy:
  strategy: blue-green
```

## Health checks

Three types of health checks are supported:

**HTTP** -- Hits an endpoint and expects a 2xx response:
```yaml
health_check:
  type: http
  endpoint: /healthz
  port: 3000
  interval: 5s
  retries: 5
```

**TCP** -- Checks if a port is accepting connections:
```yaml
health_check:
  type: tcp
  port: 5432
  interval: 3s
  retries: 10
```

**Command** -- Runs a command inside the host and checks exit code:
```yaml
health_check:
  type: command
  command: "docker exec myapp_api curl -sf localhost:3000/healthz"
  interval: 5s
  retries: 3
```

## Prerequisites

- SSH access to your hosts (key-based auth recommended)
- Docker installed on each host
- The SSH user must have permission to run `docker` commands

## License

MIT

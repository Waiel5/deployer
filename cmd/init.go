package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Waiel5/deployer/pkg/output"
)

const defaultConfig = `# deployer.yml - Container deployment configuration
# Documentation: https://github.com/Waiel5/deployer

project: myapp

ssh:
  user: deploy
  port: 22
  key_path: ~/.ssh/id_rsa
  # password: ""  # Use key-based auth instead

services:
  - name: web
    image: myregistry/myapp:latest
    hosts:
      - 10.0.1.10
      - 10.0.1.11
    ports:
      - "80:8080"
      - "443:8443"
    environment:
      NODE_ENV: production
      DATABASE_URL: postgres://db:5432/myapp
    volumes:
      - /data/myapp:/app/data
    network: bridge
    restart: unless-stopped
    deploy:
      strategy: rolling   # rolling or blue-green
      batch_size: 1
      delay: 10s
    health_check:
      enabled: true
      type: http           # http, tcp, or command
      endpoint: /health
      port: 8080
      interval: 5s
      timeout: 10s
      retries: 3

  - name: worker
    image: myregistry/myapp-worker:latest
    hosts:
      - 10.0.1.12
    environment:
      QUEUE_URL: redis://redis:6379
    deploy:
      strategy: rolling
      batch_size: 1
    health_check:
      enabled: true
      type: command
      command: "pgrep -f worker"
      interval: 5s
      retries: 5
`

func NewInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new deployer.yml config file",
		Long: `Init creates a deployer.yml configuration file in the current directory
with sensible defaults and documentation comments. Use this as a starting
point for your deployment configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			term := output.NewTerminal(verbose)

			filename := "deployer.yml"

			if _, err := os.Stat(filename); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", filename)
			}

			if err := os.WriteFile(filename, []byte(defaultConfig), 0644); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			term.Success("Created %s", filename)
			term.Info("Edit the file with your hosts, images, and services, then run:")
			term.Info("  deployer deploy")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing config")

	return cmd
}

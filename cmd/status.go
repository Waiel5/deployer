package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Waiel5/deployer/pkg/config"
	"github.com/Waiel5/deployer/pkg/docker"
	"github.com/Waiel5/deployer/pkg/output"
	"github.com/Waiel5/deployer/pkg/ssh"
)

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of deployed services",
		Long: `Status connects to each host and shows the current state of all
managed containers including their image version, uptime, ports,
and health status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")

			term := output.NewTerminal(verbose)
			term.Header("Deployer - Service Status")

			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			pool := ssh.NewConnectionPool()
			defer pool.CloseAll()

			headers := []string{"Service", "Host", "Status", "Image", "Uptime", "Ports"}
			var rows [][]string

			for _, svc := range cfg.Services {
				for _, host := range svc.Hosts {
					client, err := pool.Get(host, cfg.SSH)
					if err != nil {
						rows = append(rows, []string{
							svc.Name, host, "UNREACHABLE", "-", "-", "-",
						})
						continue
					}

					dockerClient := docker.NewClient(client)
					containerName := fmt.Sprintf("%s_%s", cfg.Project, svc.Name)

					info, err := dockerClient.Inspect(containerName)
					if err != nil {
						rows = append(rows, []string{
							svc.Name, host, "NOT FOUND", "-", "-", "-",
						})
						continue
					}

					status := "RUNNING"
					if !info.Running {
						status = "STOPPED"
					}

					ports := strings.Join(info.Ports, ", ")
					if ports == "" {
						ports = "-"
					}

					rows = append(rows, []string{
						svc.Name, host, status, info.Image, info.Uptime, ports,
					})
				}
			}

			term.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

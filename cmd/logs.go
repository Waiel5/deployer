package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Waiel5/deployer/pkg/config"
	"github.com/Waiel5/deployer/pkg/output"
	"github.com/Waiel5/deployer/pkg/ssh"
)

func NewLogsCmd() *cobra.Command {
	var (
		serviceName string
		follow      bool
		tail        int
		host        string
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail logs from remote containers",
		Long: `Logs connects to the remote hosts and streams Docker container logs.
You can follow logs in real-time or fetch the last N lines.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")

			term := output.NewTerminal(verbose)

			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if serviceName == "" {
				return fmt.Errorf("service name is required (use --service)")
			}

			var svc *config.Service
			for i := range cfg.Services {
				if cfg.Services[i].Name == serviceName {
					svc = &cfg.Services[i]
					break
				}
			}
			if svc == nil {
				return fmt.Errorf("service %q not found in config", serviceName)
			}

			targetHosts := svc.Hosts
			if host != "" {
				found := false
				for _, h := range svc.Hosts {
					if h == host {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("host %q not configured for service %s", host, svc.Name)
				}
				targetHosts = []string{host}
			}

			pool := ssh.NewConnectionPool()
			defer pool.CloseAll()

			containerName := fmt.Sprintf("%s_%s", cfg.Project, svc.Name)

			for _, h := range targetHosts {
				client, err := pool.Get(h, cfg.SSH)
				if err != nil {
					term.Error("[%s] SSH connection failed: %v", h, err)
					continue
				}

				dockerCmd := fmt.Sprintf("docker logs %s", containerName)

				var flags []string
				if follow {
					flags = append(flags, "-f")
				}
				if tail > 0 {
					flags = append(flags, fmt.Sprintf("--tail %d", tail))
				}

				if len(flags) > 0 {
					dockerCmd = fmt.Sprintf("docker logs %s %s", strings.Join(flags, " "), containerName)
				}

				if len(targetHosts) > 1 {
					term.Info("=== Logs from %s ===", h)
				}

				output, err := client.Execute(dockerCmd)
				if err != nil {
					term.Error("[%s] Failed to fetch logs: %v", h, err)
					continue
				}

				fmt.Print(output)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&serviceName, "service", "s", "", "service name (required)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().IntVarP(&tail, "tail", "n", 100, "number of lines to show")
	cmd.Flags().StringVar(&host, "host", "", "specific host to fetch logs from")
	_ = cmd.MarkFlagRequired("service")

	return cmd
}

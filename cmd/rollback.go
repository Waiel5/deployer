package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Waiel5/deployer/pkg/config"
	"github.com/Waiel5/deployer/pkg/deploy"
	"github.com/Waiel5/deployer/pkg/docker"
	"github.com/Waiel5/deployer/pkg/output"
	"github.com/Waiel5/deployer/pkg/ssh"
)

func NewRollbackCmd() *cobra.Command {
	var (
		serviceName string
		targetVer   string
	)

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback a service to its previous version",
		Long: `Rollback reverts a service to its previously deployed version.
If no version is specified, it rolls back to the immediate previous
deployment. Version history is tracked locally in .deployer/versions.json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")

			term := output.NewTerminal(verbose)
			term.Header("Deployer - Rollback")

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

			tracker := deploy.NewVersionTracker(cfg.Project)
			history := tracker.GetHistory(svc.Name)

			if len(history) < 2 && targetVer == "" {
				return fmt.Errorf("no previous version found for %s", svc.Name)
			}

			rollbackImage := targetVer
			if rollbackImage == "" {
				rollbackImage = history[len(history)-2]
			}

			term.Warn("Rolling back %s to %s", svc.Name, rollbackImage)

			pool := ssh.NewConnectionPool()
			defer pool.CloseAll()

			for _, host := range svc.Hosts {
				term.Info("  [%s] Connecting...", host)

				client, err := pool.Get(host, cfg.SSH)
				if err != nil {
					term.Error("  [%s] SSH connection failed: %v", host, err)
					return fmt.Errorf("rollback failed on %s: %w", host, err)
				}

				dockerClient := docker.NewClient(client)
				containerName := fmt.Sprintf("%s_%s", cfg.Project, svc.Name)

				term.Info("  [%s] Pulling rollback image %s", host, rollbackImage)
				if err := dockerClient.Pull(rollbackImage); err != nil {
					return fmt.Errorf("failed to pull image on %s: %w", host, err)
				}

				term.Info("  [%s] Stopping current container", host)
				_ = dockerClient.Stop(containerName, 30)

				term.Info("  [%s] Starting rollback container", host)
				runOpts := docker.RunOptions{
					Name:        containerName,
					Image:       rollbackImage,
					Ports:       svc.Ports,
					Env:         svc.Environment,
					Volumes:     svc.Volumes,
					NetworkMode: svc.Network,
					Restart:     svc.Restart,
				}

				if err := dockerClient.Run(runOpts); err != nil {
					return fmt.Errorf("failed to start rollback container on %s: %w", host, err)
				}

				if svc.HealthCheck.Enabled {
					term.Info("  [%s] Waiting for health check", host)
					checker := docker.NewHealthChecker(client, svc.HealthCheck)
					if err := checker.Wait(); err != nil {
						term.Error("  [%s] Health check failed after rollback: %v", host, err)
						return fmt.Errorf("rollback health check failed on %s: %w", host, err)
					}
					term.Success("  [%s] Health check passed", host)
				}

				term.Success("  [%s] Rolled back successfully", host)
			}

			tracker.Record(svc.Name, rollbackImage)
			term.Success("Rollback of %s to %s completed", svc.Name, rollbackImage)
			return nil
		},
	}

	cmd.Flags().StringVarP(&serviceName, "service", "s", "", "service to rollback (required)")
	cmd.Flags().StringVarP(&targetVer, "version", "t", "", "target version to rollback to")
	_ = cmd.MarkFlagRequired("service")

	return cmd
}

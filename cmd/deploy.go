package cmd

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/Waiel5/deployer/pkg/config"
	"github.com/Waiel5/deployer/pkg/deploy"
	"github.com/Waiel5/deployer/pkg/docker"
	"github.com/Waiel5/deployer/pkg/output"
	"github.com/Waiel5/deployer/pkg/ssh"
)

func NewDeployCmd() *cobra.Command {
	var (
		services []string
		force    bool
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy services to remote hosts",
		Long: `Deploy reads the deployer.yml config, connects to each host via SSH,
pulls the specified Docker images, and performs a zero-downtime rolling
update with health checks. If a health check fails, the service is
automatically rolled back.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")

			term := output.NewTerminal(verbose)
			term.Header("Deployer - Zero-downtime deployment")

			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("config validation failed: %w", err)
			}

			targetServices := filterServices(cfg, services)
			if len(targetServices) == 0 {
				return fmt.Errorf("no matching services found")
			}

			term.Info("Deploying %d service(s) across %d host(s)",
				len(targetServices), countHosts(targetServices))

			if dryRun {
				term.Warn("Dry run mode - no changes will be made")
				printDeployPlan(term, targetServices)
				return nil
			}

			pool := ssh.NewConnectionPool()
			defer pool.CloseAll()

			var deployErr error
			for _, svc := range targetServices {
				if err := deployService(term, pool, cfg, svc, force); err != nil {
					deployErr = err
					term.Error("Failed to deploy %s: %v", svc.Name, err)
					if !force {
						break
					}
				}
			}

			if deployErr != nil {
				term.Error("Deployment completed with errors")
				return deployErr
			}

			term.Success("All services deployed successfully")
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&services, "service", "s", nil, "deploy specific services (comma-separated)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "continue deploying even if a service fails")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show deployment plan without executing")

	return cmd
}

func deployService(term *output.Terminal, pool *ssh.ConnectionPool, cfg *config.Config, svc config.Service, force bool) error {
	term.StartSpinner("Deploying %s", svc.Name)

	tracker := deploy.NewVersionTracker(cfg.Project)
	currentVersion := tracker.GetCurrent(svc.Name)

	var (
		mu       sync.Mutex
		failures []string
	)

	strategy := deploy.NewStrategy(svc.Deploy.Strategy)
	hosts := svc.Hosts
	batches := strategy.Plan(hosts)

	for batchIdx, batch := range batches {
		term.Info("Rolling update batch %d/%d for %s", batchIdx+1, len(batches), svc.Name)

		var wg sync.WaitGroup
		for _, host := range batch {
			wg.Add(1)
			go func(h string) {
				defer wg.Done()

				client, err := pool.Get(h, cfg.SSH)
				if err != nil {
					mu.Lock()
					failures = append(failures, h)
					mu.Unlock()
					term.Error("  [%s] SSH connection failed: %v", h, err)
					return
				}

				dockerClient := docker.NewClient(client)

				term.Info("  [%s] Pulling image %s", h, svc.Image)
				if err := dockerClient.Pull(svc.Image); err != nil {
					mu.Lock()
					failures = append(failures, h)
					mu.Unlock()
					term.Error("  [%s] Pull failed: %v", h, err)
					return
				}

				term.Info("  [%s] Stopping old container", h)
				oldContainer := fmt.Sprintf("%s_%s", cfg.Project, svc.Name)
				_ = dockerClient.Stop(oldContainer, 30)

				term.Info("  [%s] Starting new container", h)
				runOpts := docker.RunOptions{
					Name:        oldContainer,
					Image:       svc.Image,
					Ports:       svc.Ports,
					Env:         svc.Environment,
					Volumes:     svc.Volumes,
					NetworkMode: svc.Network,
					Restart:     svc.Restart,
				}

				if err := dockerClient.Run(runOpts); err != nil {
					mu.Lock()
					failures = append(failures, h)
					mu.Unlock()
					term.Error("  [%s] Failed to start container: %v", h, err)

					if currentVersion != "" {
						term.Warn("  [%s] Rolling back to %s", h, currentVersion)
						rollbackOpts := runOpts
						rollbackOpts.Image = currentVersion
						_ = dockerClient.Run(rollbackOpts)
					}
					return
				}

				if svc.HealthCheck.Enabled {
					term.Info("  [%s] Waiting for health check", h)
					checker := docker.NewHealthChecker(client, svc.HealthCheck)
					if err := checker.Wait(); err != nil {
						mu.Lock()
						failures = append(failures, h)
						mu.Unlock()
						term.Error("  [%s] Health check failed: %v", h, err)

						term.Warn("  [%s] Rolling back due to health check failure", h)
						_ = dockerClient.Stop(oldContainer, 10)
						if currentVersion != "" {
							rollbackOpts := runOpts
							rollbackOpts.Image = currentVersion
							_ = dockerClient.Run(rollbackOpts)
						}
						return
					}
					term.Success("  [%s] Health check passed", h)
				}
			}(host)
		}

		wg.Wait()

		if len(failures) > 0 && !force {
			return fmt.Errorf("deployment failed on hosts: %v", failures)
		}

		if batchIdx < len(batches)-1 {
			delay := strategy.BatchDelay()
			term.Info("Waiting %s before next batch...", delay)
			time.Sleep(delay)
		}
	}

	if len(failures) == 0 {
		tracker.Record(svc.Name, svc.Image)
		term.StopSpinner("Deployed %s successfully", svc.Name)
	}

	return nil
}

func filterServices(cfg *config.Config, names []string) []config.Service {
	if len(names) == 0 {
		return cfg.Services
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	var result []config.Service
	for _, svc := range cfg.Services {
		if nameSet[svc.Name] {
			result = append(result, svc)
		}
	}
	return result
}

func countHosts(services []config.Service) int {
	hosts := make(map[string]bool)
	for _, svc := range services {
		for _, h := range svc.Hosts {
			hosts[h] = true
		}
	}
	return len(hosts)
}

func printDeployPlan(term *output.Terminal, services []config.Service) {
	for _, svc := range services {
		term.Info("  Service: %s", svc.Name)
		term.Info("    Image:  %s", svc.Image)
		term.Info("    Hosts:  %v", svc.Hosts)
		term.Info("    Strategy: %s", svc.Deploy.Strategy)
		if svc.HealthCheck.Enabled {
			term.Info("    Health check: %s %s (interval: %s, retries: %d)",
				svc.HealthCheck.Type, svc.HealthCheck.Endpoint,
				svc.HealthCheck.Interval, svc.HealthCheck.Retries)
		}
		fmt.Fprintln(os.Stderr)
	}
}

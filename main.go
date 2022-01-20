package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Waiel5/deployer/cmd"
)

var version = "0.4.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "deployer",
		Short: "Zero-downtime container deployments via SSH",
		Long: `Deployer is a lightweight container deployment tool that deploys Docker
containers to remote hosts via SSH with zero-downtime rolling updates,
health checks, and instant rollback.

Think of it as a simpler alternative to Kubernetes for small-to-medium
deployments where you just need containers running on a few servers.`,
		Version: version,
	}

	rootCmd.AddCommand(cmd.NewDeployCmd())
	rootCmd.AddCommand(cmd.NewRollbackCmd())
	rootCmd.AddCommand(cmd.NewStatusCmd())
	rootCmd.AddCommand(cmd.NewInitCmd())
	rootCmd.AddCommand(cmd.NewLogsCmd())

	rootCmd.PersistentFlags().StringP("config", "c", "deployer.yml", "path to config file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"os"
	"sync"

	"github.com/nace/brezno/internal/cli"
	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	quiet   bool
	noColor bool
	debug   bool

	ctx  *cli.GlobalContext
	once sync.Once
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "brezno",
	Short: "Brezno - dm-crypt container manager",
	Long: `Brezno is a CLI utility for managing LUKS2 encrypted containers on Linux.

It provides a simple interface for creating, mounting, and managing
dm-crypt containers similar to VeraCrypt but CLI-only and using
standard Linux encryption tools (cryptsetup, dm-crypt).`,
	Version: "0.1.0",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Update context components with parsed flag values
		once.Do(func() {
			// Recreate executor and logger with parsed flags
			ctx.Executor = system.NewExecutor(debug)
			ctx.Logger = ui.NewLogger(verbose, quiet, noColor)

			// Recreate managers with new executor
			ctx.LoopManager = container.NewLoopManager(ctx.Executor)
			ctx.LUKSManager = container.NewLUKSManager(ctx.Executor)
			ctx.MountMgr = container.NewMountManager(ctx.Executor)
			ctx.Discovery = container.NewDiscovery(ctx.Executor)
		})
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode (suppress non-error output)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Debug mode (show commands)")

	// Create initial context with default values
	// Will be updated in PersistentPreRun with parsed flag values
	ctx = cli.NewGlobalContext(false, false, false, false)

	// Register commands
	rootCmd.AddCommand(cli.NewCreateCommand(ctx))
	rootCmd.AddCommand(cli.NewMountCommand(ctx))
	rootCmd.AddCommand(cli.NewUnmountCommand(ctx))
	rootCmd.AddCommand(cli.NewListCommand(ctx))
	rootCmd.AddCommand(cli.NewResizeCommand(ctx))
	rootCmd.AddCommand(cli.NewPasswordCommand(ctx))

	// Set up help templates
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

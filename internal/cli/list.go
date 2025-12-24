package cli

import (
	"fmt"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// ListCommand handles listing containers
type ListCommand struct {
	ctx     *GlobalContext
	verbose bool
	json    bool
}

// NewListCommand creates the list command
func NewListCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &ListCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "list",
		Short: "List active encrypted containers",
		Long:  `List all currently mounted LUKS encrypted containers.`,
		RunE:  cmd.Run,
	}

	cobraCmd.Flags().BoolVarP(&cmd.verbose, "verbose", "v", false, "Verbose output")
	cobraCmd.Flags().BoolVarP(&cmd.json, "json", "j", false, "JSON output")

	return cobraCmd
}

// Run executes the list command
func (c *ListCommand) Run(cmd *cobra.Command, args []string) error {
	if err := system.RequireRoot(); err != nil {
		return err
	}

	if err := c.ctx.CheckDependencies(); err != nil {
		return err
	}

	// Discover active containers
	containers, err := c.ctx.Discovery.DiscoverActive()
	if err != nil {
		return fmt.Errorf("failed to discover containers: %w", err)
	}

	if len(containers) == 0 {
		fmt.Println("No active containers found")
		return nil
	}

	// Output based on format
	if c.json {
		return ui.PrintJSON(containers)
	}

	if c.verbose {
		c.printVerbose(containers)
	} else {
		c.printTable(containers)
	}

	return nil
}

func (c *ListCommand) printTable(containers []container.Container) {
	table := ui.NewTable("CONTAINER", "MAPPER", "MOUNT POINT", "SIZE", "USED")

	for _, cont := range containers {
		size := "-"
		used := "-"
		if cont.Size > 0 {
			size = system.FormatSize(cont.Size)
			used = system.FormatSize(cont.Used)
		}

		mountPoint := cont.MountPoint
		if mountPoint == "" {
			mountPoint = "-"
		}

		table.AddRow(
			cont.Path,
			cont.MapperName,
			mountPoint,
			size,
			used,
		)
	}

	table.Print()
}

func (c *ListCommand) printVerbose(containers []container.Container) {
	for i, cont := range containers {
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("Container: %s\n", cont.Path)
		fmt.Printf("  Mapper: %s\n", cont.MapperName)

		if cont.MountPoint != "" {
			fmt.Printf("  Mount Point: %s\n", cont.MountPoint)
		}

		if cont.LoopDevice != "" {
			fmt.Printf("  Loop Device: %s\n", cont.LoopDevice)
		}

		if cont.Filesystem != "" {
			fmt.Printf("  Filesystem: %s\n", cont.Filesystem)
		}

		if cont.Size > 0 {
			fmt.Printf("  Size: %s\n", system.FormatSize(cont.Size))
			fmt.Printf("  Used: %s", system.FormatSize(cont.Used))
			if cont.Size > 0 {
				percentage := float64(cont.Used) / float64(cont.Size) * 100
				fmt.Printf(" (%.1f%%)\n", percentage)
			} else {
				fmt.Println()
			}

			if cont.Size > cont.Used {
				available := cont.Size - cont.Used
				fmt.Printf("  Available: %s\n", system.FormatSize(available))
			}
		}
	}
}

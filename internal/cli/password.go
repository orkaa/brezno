package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// PasswordCommand handles changing container authentication
type PasswordCommand struct {
	ctx           *GlobalContext
	keyfile       string
	newKeyfile    string
	passwordStdin bool
}

// NewPasswordCommand creates a new password command
func NewPasswordCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &PasswordCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "password <container-path>",
		Short: "Change LUKS container passphrase or keyfile",
		Long: `Change the authentication method for a LUKS container.

Supports all transitions:
  - Password to password (no flags)
  - Password to keyfile (--new-keyfile only)
  - Keyfile to password (--keyfile only)
  - Keyfile to keyfile (--keyfile and --new-keyfile)

The container must be unmounted before changing credentials.`,
		Args: cobra.MaximumNArgs(1),
		RunE: cmd.Run,
	}

	cobraCmd.Flags().StringVarP(&cmd.keyfile, "keyfile", "k", "",
		"Current keyfile path (if not set, will prompt for current passphrase)")
	cobraCmd.Flags().StringVar(&cmd.newKeyfile, "new-keyfile", "",
		"New keyfile path (if not set, will prompt for new passphrase)")
	cobraCmd.Flags().BoolVar(&cmd.passwordStdin, "password-stdin", false,
		"Read passphrases from stdin (for automation)")

	return cobraCmd
}

// Run executes the password command
func (c *PasswordCommand) Run(cmd *cobra.Command, args []string) error {
	// Check root permissions
	if err := system.RequireRoot(); err != nil {
		return err
	}

	// Check dependencies
	if err := c.ctx.CheckDependencies(); err != nil {
		return err
	}

	// Get container path (from args or prompt)
	var containerPath string
	if len(args) > 0 {
		containerPath = args[0]
	} else {
		containerPath = ui.PromptString("Container file path")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	containerPath = absPath

	// Verify container file exists
	if _, err := os.Stat(containerPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("container file not found: %s", containerPath)
		}
		return fmt.Errorf("failed to access container: %w", err)
	}

	// Verify it's a LUKS container
	isLuks, err := c.ctx.LUKSManager.IsLUKS(containerPath)
	if err != nil {
		return fmt.Errorf("failed to check LUKS format: %w", err)
	}
	if !isLuks {
		return fmt.Errorf("not a LUKS container: %s", containerPath)
	}

	// Verify container is NOT mounted
	existing, err := c.ctx.Discovery.FindByPath(containerPath)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("container must be unmounted before changing credentials\n"+
			"Currently mounted at: %s\n"+
			"Run 'brezno unmount %s' first", existing.MountPoint, containerPath)
	}

	// Get current authentication method
	c.ctx.Logger.Info("Enter current authentication credentials:")
	currentAuth, err := GetAuthMethod(c.keyfile, false, c.passwordStdin, "", "")
	if err != nil {
		return fmt.Errorf("failed to get current authentication: %w", err)
	}
	// Ensure password cleanup
	if pwAuth, ok := currentAuth.(*container.PasswordAuth); ok {
		defer pwAuth.Password.Zeroize()
	}

	// Get new authentication method
	c.ctx.Logger.Info("Enter new authentication credentials:")
	newAuth, err := GetAuthMethod(c.newKeyfile, true, c.passwordStdin, "Enter new passphrase", "Confirm new passphrase")
	if err != nil {
		return fmt.Errorf("failed to get new authentication: %w", err)
	}
	// Ensure password cleanup
	if pwAuth, ok := newAuth.(*container.PasswordAuth); ok {
		defer pwAuth.Password.Zeroize()
	}

	// Execute password change
	return c.execute(containerPath, currentAuth, newAuth)
}

// execute performs the credential change operation
func (c *PasswordCommand) execute(path string, currentAuth, newAuth container.AuthMethod) error {
	c.ctx.Logger.Info("Changing container credentials...")

	if err := c.ctx.LUKSManager.ChangeKey(path, currentAuth, newAuth); err != nil {
		// Provide helpful error messages for common failures
		if strings.Contains(err.Error(), "No key available") {
			return fmt.Errorf("incorrect current passphrase or keyfile")
		}
		return fmt.Errorf("failed to change credentials: %w", err)
	}

	c.ctx.Logger.Success("Container credentials changed successfully")
	c.ctx.Logger.Info("Container: %s", path)

	// Display helpful message about what changed
	oldType := "password"
	newType := "password"
	if _, ok := currentAuth.(*container.KeyfileAuth); ok {
		oldType = "keyfile"
	}
	if _, ok := newAuth.(*container.KeyfileAuth); ok {
		newType = "keyfile"
	}

	if oldType != newType {
		c.ctx.Logger.Info("Authentication changed: %s → %s", oldType, newType)
	}

	return nil
}

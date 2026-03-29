package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
	"github.com/spf13/cobra"
)

// VerifyResult contains the verification results
type VerifyResult struct {
	ContainerPath   string                   `json:"container_path"`
	HeaderValid     bool                     `json:"header_valid"`
	HeaderInfo      *container.LUKSDumpInfo  `json:"header_info,omitempty"`
	PassphraseValid *bool                    `json:"passphrase_valid,omitempty"`
	Errors          []string                 `json:"errors,omitempty"`
}

// VerifyCommand handles LUKS container verification
type VerifyCommand struct {
	ctx        *GlobalContext
	full       bool
	keyfile    string
	jsonOutput bool
}

// NewVerifyCommand creates the verify command
func NewVerifyCommand(ctx *GlobalContext) *cobra.Command {
	cmd := &VerifyCommand{ctx: ctx}

	cobraCmd := &cobra.Command{
		Use:   "verify <container-path>",
		Short: "Verify LUKS container integrity",
		Long: `Verify the integrity of a LUKS encrypted container.

By default, only validates the LUKS header structure without requiring
a password. Use --full to also verify that credentials are valid.

Header verification checks:
  - LUKS magic bytes and version
  - Header structure and metadata
  - Key slot information

Full verification (--full) additionally:
  - Tests passphrase without opening the container
  - Verifies key derivation succeeds`,
		Args: cobra.MaximumNArgs(1),
		RunE: cmd.Run,
	}

	cobraCmd.Flags().BoolVarP(&cmd.full, "full", "f", false, "Verify credentials in addition to header")
	cobraCmd.Flags().StringVarP(&cmd.keyfile, "keyfile", "k", "", "Keyfile path for authentication (with --full)")
	cobraCmd.Flags().BoolVarP(&cmd.jsonOutput, "json", "j", false, "Output results in JSON format")

	return cobraCmd
}

// Run executes the verify command
func (c *VerifyCommand) Run(cmd *cobra.Command, args []string) error {
	if err := system.RequireRoot(); err != nil {
		return err
	}

	if err := c.ctx.CheckDependencies(); err != nil {
		return err
	}

	// Get container path
	var containerPath string
	if len(args) > 0 {
		containerPath = args[0]
	} else {
		if c.jsonOutput {
			return fmt.Errorf("container path is required in JSON mode")
		}
		containerPath = ui.PromptString("Container file path")
	}

	// Resolve to absolute path and follow symlinks
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return c.outputError(containerPath, fmt.Errorf("invalid path: %w", err))
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return c.outputError(containerPath, fmt.Errorf("failed to resolve path: %w", err))
	}
	containerPath = resolvedPath

	// Verify container exists
	if _, err := os.Stat(containerPath); err != nil {
		if os.IsNotExist(err) {
			return c.outputError(containerPath, fmt.Errorf("container not found: %s", containerPath))
		}
		return c.outputError(containerPath, fmt.Errorf("failed to access container: %w", err))
	}

	if !c.jsonOutput {
		c.ctx.Logger.Info("Verifying container: %s", containerPath)
	}

	return c.execute(containerPath)
}

func (c *VerifyCommand) execute(containerPath string) error {
	result := &VerifyResult{
		ContainerPath: containerPath,
	}

	// Step 1: Check if it's a LUKS container
	isLuks, err := c.ctx.LUKSManager.IsLUKS(containerPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to check LUKS format: %v", err))
		return c.output(result)
	}
	if !isLuks {
		result.Errors = append(result.Errors, "not a valid LUKS container")
		return c.output(result)
	}

	// Step 2: Get header information (validates header integrity)
	info, err := c.ctx.LUKSManager.Dump(containerPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("header validation failed: %v", err))
		return c.output(result)
	}

	result.HeaderValid = true
	result.HeaderInfo = info

	// Step 3: If --full, verify credentials
	if c.full {
		auth, err := GetAuthMethod(c.keyfile, false, "", "")
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to get credentials: %v", err))
			return c.output(result)
		}
		// Ensure password is zeroized when done
		if pwAuth, ok := auth.(*container.PasswordAuth); ok {
			defer pwAuth.Password.Zeroize()
		}

		err = c.ctx.LUKSManager.TestPassphrase(containerPath, auth)
		valid := err == nil
		result.PassphraseValid = &valid
		if err != nil {
			result.Errors = append(result.Errors, "passphrase verification failed")
		}
	}

	return c.output(result)
}

func (c *VerifyCommand) output(result *VerifyResult) error {
	if c.jsonOutput {
		return c.outputJSON(result)
	}
	return c.outputHuman(result)
}

func (c *VerifyCommand) outputJSON(result *VerifyResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Return error if verification failed
	if len(result.Errors) > 0 {
		return fmt.Errorf("verification failed")
	}
	return nil
}

func (c *VerifyCommand) outputHuman(result *VerifyResult) error {
	if !result.HeaderValid {
		for _, err := range result.Errors {
			c.ctx.Logger.Error("%s", err)
		}
		return fmt.Errorf("header verification failed")
	}

	// Display header info
	c.ctx.Logger.Info("")
	c.ctx.Logger.Info("Header Verification:")
	if result.HeaderInfo != nil {
		c.ctx.Logger.Info("  LUKS Version: %s", result.HeaderInfo.Version)
		c.ctx.Logger.Info("  UUID: %s", result.HeaderInfo.UUID)
		if result.HeaderInfo.Cipher != "" && result.HeaderInfo.CipherMode != "" {
			c.ctx.Logger.Info("  Cipher: %s-%s", result.HeaderInfo.Cipher, result.HeaderInfo.CipherMode)
		}
		if result.HeaderInfo.HashSpec != "" {
			c.ctx.Logger.Info("  Hash: %s", result.HeaderInfo.HashSpec)
		}
		if len(result.HeaderInfo.KeySlots) > 0 {
			slots := make([]string, len(result.HeaderInfo.KeySlots))
			for i, slot := range result.HeaderInfo.KeySlots {
				slots[i] = fmt.Sprintf("%d", slot)
			}
			c.ctx.Logger.Info("  Active Key Slots: %s", strings.Join(slots, ", "))
		}
	}

	c.ctx.Logger.Info("")
	c.ctx.Logger.Success("Container header is valid")

	// Display passphrase verification result if --full was used
	if result.PassphraseValid != nil {
		c.ctx.Logger.Info("")
		if *result.PassphraseValid {
			c.ctx.Logger.Success("Passphrase verification successful")
		} else {
			c.ctx.Logger.Error("Passphrase verification failed")
			return fmt.Errorf("passphrase verification failed")
		}
	}

	return nil
}

func (c *VerifyCommand) outputError(containerPath string, err error) error {
	if c.jsonOutput {
		result := &VerifyResult{
			ContainerPath: containerPath,
			Errors:        []string{err.Error()},
		}
		return c.outputJSON(result)
	}
	return err
}

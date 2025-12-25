package cli

import (
	"bytes"
	"fmt"

	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
)

// GlobalContext holds shared resources for all commands
type GlobalContext struct {
	Executor    *system.Executor
	Logger      *ui.Logger
	LoopManager *container.LoopManager
	LUKSManager *container.LUKSManager
	MountMgr    *container.MountManager
	Discovery   *container.Discovery
}

// NewGlobalContext creates a new global context
func NewGlobalContext(verbose, quiet, noColor, debug bool) *GlobalContext {
	executor := system.NewExecutor(debug)
	logger := ui.NewLogger(verbose, quiet, noColor)

	return &GlobalContext{
		Executor:    executor,
		Logger:      logger,
		LoopManager: container.NewLoopManager(executor),
		LUKSManager: container.NewLUKSManager(executor),
		MountMgr:    container.NewMountManager(executor),
		Discovery:   container.NewDiscovery(executor),
	}
}

// CheckDependencies checks for required system commands
func (ctx *GlobalContext) CheckDependencies() error {
	deps := []string{
		"cryptsetup",
		"losetup",
		"mount",
		"umount",
		"dmsetup",
		"df",
	}
	return ctx.Executor.CheckDependencies(deps)
}

// GetAuthMethod determines the authentication method based on keyfile flag.
// If requireConfirmation is true, prompts for password confirmation (for create operations).
// Caller is responsible for calling Zeroize() on PasswordAuth.Password when done.
func GetAuthMethod(keyfile string, requireConfirmation bool) (container.AuthMethod, error) {
	if keyfile != "" {
		// Validate and resolve keyfile path
		resolvedKeyfile, err := system.ValidateKeyfilePath(keyfile)
		if err != nil {
			return nil, err
		}
		return &container.KeyfileAuth{KeyfilePath: resolvedKeyfile}, nil
	}

	// Prompt for password
	password, err := ui.PromptPassword("Enter passphrase")
	if err != nil {
		return nil, fmt.Errorf("failed to read passphrase: %w", err)
	}

	if requireConfirmation {
		confirmPassword, err := ui.PromptPassword("Confirm passphrase")
		if err != nil {
			password.Zeroize()
			return nil, fmt.Errorf("failed to read passphrase: %w", err)
		}
		defer confirmPassword.Zeroize()

		if !bytes.Equal(password.Bytes(), confirmPassword.Bytes()) {
			password.Zeroize()
			return nil, fmt.Errorf("passphrases don't match")
		}
	}

	return &container.PasswordAuth{Password: password}, nil
}

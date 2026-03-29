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
func NewGlobalContext(verbose, quiet, noColor bool) *GlobalContext {
	executor := system.NewExecutor(verbose)
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

// GetAuthMethod determines the authentication method based on the keyfile flag.
// If requireConfirmation is true, prompts for password confirmation (for create operations).
// promptText and confirmText allow customizing the password prompts (empty string = use defaults).
// Automatically detects whether stdin is a terminal or pipe — no --password-stdin flag needed.
// Caller is responsible for calling Zeroize() on PasswordAuth.Password when done.
func GetAuthMethod(keyfile string, requireConfirmation bool, promptText string, confirmText string) (container.AuthMethod, error) {
	if keyfile != "" {
		resolvedKeyfile, err := system.ValidateKeyfilePath(keyfile)
		if err != nil {
			return nil, err
		}
		return &container.KeyfileAuth{KeyfilePath: resolvedKeyfile}, nil
	}

	if promptText == "" {
		promptText = "Enter password"
	}
	if confirmText == "" {
		confirmText = "Confirm password"
	}

	password, err := ui.GetPassword(promptText)
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}

	if requireConfirmation {
		confirmPassword, err := ui.GetPassword(confirmText)
		if err != nil {
			password.Zeroize()
			return nil, fmt.Errorf("failed to read password: %w", err)
		}
		defer confirmPassword.Zeroize()

		if !bytes.Equal(password.Bytes(), confirmPassword.Bytes()) {
			password.Zeroize()
			return nil, fmt.Errorf("passwords don't match")
		}
	}

	return &container.PasswordAuth{Password: password}, nil
}

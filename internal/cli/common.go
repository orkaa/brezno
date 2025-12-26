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

// GetAuthMethod determines the authentication method based on keyfile flag.
// If requireConfirmation is true, prompts for password confirmation (for create operations).
// If passwordStdin is true, reads password from stdin instead of prompting.
// promptText and confirmText allow customizing the password prompts (empty string = use defaults).
// Caller is responsible for calling Zeroize() on PasswordAuth.Password when done.
func GetAuthMethod(keyfile string, requireConfirmation bool, passwordStdin bool, promptText string, confirmText string) (container.AuthMethod, error) {
	if keyfile != "" {
		// Validate and resolve keyfile path
		resolvedKeyfile, err := system.ValidateKeyfilePath(keyfile)
		if err != nil {
			return nil, err
		}
		return &container.KeyfileAuth{KeyfilePath: resolvedKeyfile}, nil
	}

	// Use defaults if not specified
	if promptText == "" {
		promptText = "Enter passphrase"
	}
	if confirmText == "" {
		confirmText = "Confirm passphrase"
	}

	var password *system.SecureBytes
	var err error

	if passwordStdin {
		// Read password from stdin
		password, err = ui.ReadPasswordFromStdin()
		if err != nil {
			return nil, fmt.Errorf("failed to read passphrase from stdin: %w", err)
		}
	} else {
		// Prompt for password
		password, err = ui.PromptPassword(promptText)
		if err != nil {
			return nil, fmt.Errorf("failed to read passphrase: %w", err)
		}
	}

	if requireConfirmation {
		var confirmPassword *system.SecureBytes
		if passwordStdin {
			// Read confirmation from stdin
			confirmPassword, err = ui.ReadPasswordFromStdin()
			if err != nil {
				password.Zeroize()
				return nil, fmt.Errorf("failed to read passphrase confirmation from stdin: %w", err)
			}
		} else {
			confirmPassword, err = ui.PromptPassword(confirmText)
			if err != nil {
				password.Zeroize()
				return nil, fmt.Errorf("failed to read passphrase: %w", err)
			}
		}
		defer confirmPassword.Zeroize()

		if !bytes.Equal(password.Bytes(), confirmPassword.Bytes()) {
			password.Zeroize()
			return nil, fmt.Errorf("passphrases don't match")
		}
	}

	return &container.PasswordAuth{Password: password}, nil
}

package system

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Executor handles execution of external commands
type Executor struct {
	dryRun bool
	debug  bool
}

// NewExecutor creates a new executor
func NewExecutor(debug bool) *Executor {
	return &Executor{
		dryRun: false,
		debug:  debug,
	}
}

// Run executes a command and discards output
func (e *Executor) Run(name string, args ...string) error {
	_, err := e.RunOutput(name, args...)
	return err
}

// RunOutput executes a command and returns stdout
func (e *Executor) RunOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	return e.RunCmd(cmd)
}

// sanitizeCommand returns a sanitized command string for logging,
// redacting sensitive arguments like keyfile paths
func (e *Executor) sanitizeCommand(cmd *exec.Cmd) string {
	if cmd == nil || len(cmd.Args) == 0 {
		return ""
	}

	args := make([]string, len(cmd.Args))
	copy(args, cmd.Args)

	// Redact arguments following sensitive flags
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--key-file" || args[i] == "-k" {
			args[i+1] = "[REDACTED]"
		}
	}

	// For luksChangeKey, also redact the positional keyfile argument (if present)
	// Format: cryptsetup luksChangeKey --key-slot 0 <device> [<new keyfile>]
	if len(args) >= 2 && args[1] == "luksChangeKey" {
		// If there are more than the expected args, last one is likely the new keyfile
		if len(args) > 5 { // cryptsetup luksChangeKey --key-slot 0 device [keyfile]
			args[len(args)-1] = "[REDACTED]"
		}
	}

	result := strings.Join(args, " ")

	// Indicate if stdin is being used (potential password input)
	if cmd.Stdin != nil && cmd.Stdin != os.Stdin {
		result += " < [STDIN]"
	}

	return result
}

// RunCmd executes a prepared command
func (e *Executor) RunCmd(cmd *exec.Cmd) (string, error) {
	if e.dryRun {
		fmt.Printf("[DRY RUN] %s\n", e.sanitizeCommand(cmd))
		return "", nil
	}

	if e.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Executing: %s\n", e.sanitizeCommand(cmd))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w\nStderr: %s",
			cmd.Args[0], err, stderr.String())
	}

	return stdout.String(), nil
}

// CommandExists checks if a command is available in PATH
func (e *Executor) CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// CheckDependencies verifies required commands are available
func (e *Executor) CheckDependencies(deps []string) error {
	var missing []string
	for _, dep := range deps {
		if !e.CommandExists(dep) {
			missing = append(missing, dep)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required commands: %s",
			strings.Join(missing, ", "))
	}
	return nil
}

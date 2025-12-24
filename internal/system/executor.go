package system

import (
	"bytes"
	"fmt"
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

// RunCmd executes a prepared command
func (e *Executor) RunCmd(cmd *exec.Cmd) (string, error) {
	if e.dryRun {
		fmt.Printf("[DRY RUN] %s\n", cmd.String())
		return "", nil
	}

	if e.debug {
		fmt.Printf("[DEBUG] Executing: %s\n", cmd.String())
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

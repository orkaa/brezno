package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/nace/brezno/internal/system"
	"golang.org/x/term"
)

// PromptString prompts for a string input
func PromptString(prompt string) string {
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// PromptStringWithDefault prompts for a string with a default value
func PromptStringWithDefault(prompt, defaultValue string) string {
	fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, defaultValue)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

// PromptPassword prompts for a password without echoing
func PromptPassword(prompt string) (*system.SecureBytes, error) {
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // New line after password input
	if err != nil {
		return nil, err
	}
	return system.NewSecureBytes(password), nil
}

// ReadPasswordFromStdin reads a password from stdin (for automation/testing)
// The password should be provided as a single line.
// This is useful for scripting and CI/CD pipelines.
func ReadPasswordFromStdin() (*system.SecureBytes, error) {
	var password []byte
	var b [1]byte

	for {
		n, err := os.Stdin.Read(b[:])
		if err != nil {
			if len(password) > 0 {
				// Return what we have if we hit EOF after reading something
				break
			}
			return nil, err
		}
		if n == 0 {
			break
		}
		if b[0] == '\n' {
			break
		}
		if b[0] != '\r' { // Skip carriage return
			password = append(password, b[0])
		}
	}

	return system.NewSecureBytes(password), nil
}

// PromptConfirm prompts for yes/no confirmation
func PromptConfirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

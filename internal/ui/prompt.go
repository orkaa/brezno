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

// PromptConfirm prompts for yes/no confirmation
func PromptConfirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

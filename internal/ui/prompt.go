package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nace/brezno/internal/system"
	"golang.org/x/term"
)

// maxPasswordSize is the maximum password length in bytes, matching LUKS2's compiled-in limit.
const maxPasswordSize = 512

// GetPassword reads a password securely. If stdin is a terminal, it prompts
// interactively with echo disabled. If stdin is a pipe, it reads one line
// directly into a pre-allocated buffer — no intermediate copies, no append
// reallocations. Returns an error if the password exceeds 512 bytes.
func GetPassword(prompt string) (*system.SecureBytes, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, err
		}
		sb := system.NewSecureBytes(password)
		if sb.Len() > maxPasswordSize {
			sb.Zeroize()
			return nil, fmt.Errorf("password exceeds %d byte maximum", maxPasswordSize)
		}
		return sb, nil
	}

	// Pipe path: read directly into a pre-allocated fixed-size buffer so no
	// append reallocations occur and there are no stale partial copies in memory.
	buf := make([]byte, maxPasswordSize)
	n := 0
	var tmp [1]byte
	for {
		_, err := os.Stdin.Read(tmp[:])
		if err == io.EOF || tmp[0] == '\n' {
			break
		}
		if err != nil {
			for i := range buf {
				buf[i] = 0
			}
			return nil, fmt.Errorf("failed to read password: %w", err)
		}
		if tmp[0] == '\r' {
			continue
		}
		if n >= maxPasswordSize {
			for i := range buf {
				buf[i] = 0
			}
			return nil, fmt.Errorf("password exceeds %d byte maximum", maxPasswordSize)
		}
		buf[n] = tmp[0]
		n++
	}

	return system.NewSecureBytes(buf[:n]), nil
}

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

// PromptConfirm prompts for yes/no confirmation
func PromptConfirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

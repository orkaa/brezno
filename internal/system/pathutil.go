package system

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ValidateKeyfilePath validates and resolves a keyfile path, checking for
// security issues like symlinks, incorrect file types, and insecure permissions.
// Returns the canonical absolute path if valid.
func ValidateKeyfilePath(path string) (string, error) {
	// Resolve symlinks to canonical path
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("keyfile not found: %s", path)
		}
		return "", fmt.Errorf("failed to resolve keyfile path: %w", err)
	}

	// Clean the path (remove . and ..)
	resolved = filepath.Clean(resolved)

	// Get file info
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("keyfile not accessible: %w", err)
	}

	// Verify it's a regular file (not directory, device, socket, etc.)
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("keyfile must be a regular file, not a directory or device: %s", resolved)
	}

	// Check permissions - warn if readable by group or others
	mode := info.Mode().Perm()
	if mode&0044 != 0 {
		fmt.Fprintf(os.Stderr, "\033[1;33mWARNING:\033[0m Keyfile %s has insecure permissions (%04o)\n", resolved, mode)
		fmt.Fprintf(os.Stderr, "         The keyfile is readable by group or others.\n")
		fmt.Fprintf(os.Stderr, "         Consider: chmod 600 %s\n\n", resolved)
	}

	return resolved, nil
}

// GetFileSize returns the size of a file in bytes
func GetFileSize(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	return uint64(info.Size()), nil
}

// GetAvailableSpace returns available space in bytes for the filesystem containing path
func GetAvailableSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(path), &stat); err != nil {
		return 0, fmt.Errorf("failed to get filesystem stats: %w", err)
	}
	// Available blocks * block size
	return stat.Bavail * uint64(stat.Bsize), nil
}

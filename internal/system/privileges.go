package system

import (
	"fmt"
	"os"
)

// IsRoot checks if running as root
func IsRoot() bool {
	return os.Geteuid() == 0
}

// RequireRoot ensures the program is running as root
func RequireRoot() error {
	if !IsRoot() {
		return fmt.Errorf("this command must be run as root (try with sudo)")
	}
	return nil
}

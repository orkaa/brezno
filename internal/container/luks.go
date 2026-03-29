package container

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/nace/brezno/internal/system"
)

// LUKSDumpInfo contains metadata from a LUKS container header
type LUKSDumpInfo struct {
	Version    string `json:"version"`
	UUID       string `json:"uuid"`
	Cipher     string `json:"cipher"`
	CipherMode string `json:"cipher_mode"`
	HashSpec   string `json:"hash_spec"`
	KeySlots   []int  `json:"key_slots"`
}

// AuthMethod represents a method to authenticate to a LUKS container
type AuthMethod interface {
	Apply(cmd *exec.Cmd) error
}

// PasswordAuth authenticates using a password
type PasswordAuth struct {
	Password *system.SecureBytes
}

// Apply applies password authentication to a command.
// Creates a pipe and writes the password directly via syscall, so no intermediate
// userspace buffer is allocated. The exec package detects *os.File on cmd.Stdin
// and connects it directly to the child's stdin without spawning an io.Copy goroutine.
// Caller must close cmd.Stdin after the command completes (use closeStdinFile).
func (a *PasswordAuth) Apply(cmd *exec.Cmd) error {
	if a.Password == nil {
		return fmt.Errorf("password is nil")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	if _, err := pw.Write(a.Password.Bytes()); err != nil {
		pr.Close()
		pw.Close()
		return fmt.Errorf("failed to write password to pipe: %w", err)
	}
	pw.Write([]byte{'\n'})
	pw.Close()
	cmd.Stdin = pr
	return nil
}

// KeyfileAuth authenticates using a keyfile
type KeyfileAuth struct {
	KeyfilePath string
}

// Apply applies keyfile authentication to a command
func (a *KeyfileAuth) Apply(cmd *exec.Cmd) error {
	cmd.Args = append(cmd.Args, "--key-file", a.KeyfilePath)
	return nil
}

// LUKSManager handles LUKS operations
type LUKSManager struct {
	executor *system.Executor
}

// NewLUKSManager creates a new LUKS manager
func NewLUKSManager(executor *system.Executor) *LUKSManager {
	return &LUKSManager{
		executor: executor,
	}
}

// closeStdinFile closes cmd.Stdin if it is an *os.File created by PasswordAuth.Apply.
// Must be called after RunCmd returns to avoid leaking the pipe read end in the parent.
func closeStdinFile(cmd *exec.Cmd) {
	if f, ok := cmd.Stdin.(*os.File); ok {
		f.Close()
	}
}

// Format formats a device as LUKS2
func (m *LUKSManager) Format(path string, auth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "luksFormat", "--type", "luks2", path)
	if err := auth.Apply(cmd); err != nil {
		return err
	}
	defer closeStdinFile(cmd)

	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("failed to format LUKS container: %w", err)
	}

	return nil
}

// IsLUKS checks if a file is LUKS formatted
func (m *LUKSManager) IsLUKS(path string) (bool, error) {
	err := m.executor.Run("cryptsetup", "isLuks", path)
	return err == nil, nil
}

// Open opens a LUKS container
func (m *LUKSManager) Open(device, mapperName string, auth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "luksOpen", device, mapperName)
	if err := auth.Apply(cmd); err != nil {
		return err
	}
	defer closeStdinFile(cmd)

	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("failed to open LUKS container: %w", err)
	}

	return nil
}

// Close closes a LUKS container
func (m *LUKSManager) Close(mapperName string) error {
	err := m.executor.Run("cryptsetup", "luksClose", mapperName)
	if err != nil {
		return fmt.Errorf("failed to close LUKS container %s: %w", mapperName, err)
	}
	return nil
}

// Resize expands a LUKS container to use all available space on its device
// The mapper must already be open. This requires authentication.
func (m *LUKSManager) Resize(mapperName string, auth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "resize", mapperName)
	if err := auth.Apply(cmd); err != nil {
		return err
	}
	defer closeStdinFile(cmd)

	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("failed to resize LUKS container: %w", err)
	}

	return nil
}

// GetLUKSSize gets the current size of a LUKS container in bytes
func (m *LUKSManager) GetLUKSSize(mapperName string) (uint64, error) {
	mapperDevice := "/dev/mapper/" + mapperName
	output, err := m.executor.RunOutput("blockdev", "--getsize64", mapperDevice)
	if err != nil {
		return 0, fmt.Errorf("failed to get LUKS size: %w", err)
	}

	var size uint64
	_, err = fmt.Sscanf(fmt.Sprintf("%s", output), "%d", &size)
	if err != nil {
		return 0, fmt.Errorf("failed to parse LUKS size: %w", err)
	}

	return size, nil
}

// applyNewAuth applies new authentication method to a command.
// This is different from AuthMethod.Apply() because cryptsetup luksChangeKey
// uses a positional argument for the new keyfile, not a flag.
// Only called for non-password→password transitions; password→password is
// handled directly in ChangeKey to write both passwords to a single pipe.
func applyNewAuth(cmd *exec.Cmd, auth AuthMethod) error {
	switch a := auth.(type) {
	case *KeyfileAuth:
		// Add new keyfile as positional argument
		// cryptsetup luksChangeKey <device> [<new key file>]
		cmd.Args = append(cmd.Args, a.KeyfilePath)
		return nil

	case *PasswordAuth:
		// Reached only for keyfile→password: current auth used --key-file,
		// so cmd.Stdin is nil and we create a fresh pipe for the new password.
		if a.Password == nil {
			return fmt.Errorf("password is nil")
		}
		pr, pw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		if _, err := pw.Write(a.Password.Bytes()); err != nil {
			pr.Close()
			pw.Close()
			return fmt.Errorf("failed to write password to pipe: %w", err)
		}
		pw.Write([]byte{'\n'})
		pw.Close()
		cmd.Stdin = pr
		return nil

	default:
		return fmt.Errorf("unsupported authentication type: %T", auth)
	}
}

// BackupHeader backs up the LUKS header to a file.
// No authentication is required for header backup.
func (m *LUKSManager) BackupHeader(path, outputPath string) error {
	_, err := m.executor.RunOutput("cryptsetup", "luksHeaderBackup", path, "--header-backup-file", outputPath)
	if err != nil {
		return fmt.Errorf("failed to backup LUKS header: %w", err)
	}
	return nil
}

// RestoreHeader restores a LUKS header from a backup file.
// No authentication is required for header restore.
// WARNING: Restoring the wrong header makes data permanently inaccessible.
func (m *LUKSManager) RestoreHeader(path, backupPath string) error {
	_, err := m.executor.RunOutput("cryptsetup", "luksHeaderRestore", path, "--header-backup-file", backupPath)
	if err != nil {
		return fmt.Errorf("failed to restore LUKS header: %w", err)
	}
	return nil
}

// ChangeKey changes the authentication credentials for LUKS key slot 0.
// Supports all authentication transitions:
//   - password → password
//   - password → keyfile
//   - keyfile → password
//   - keyfile → keyfile
func (m *LUKSManager) ChangeKey(device string, currentAuth, newAuth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "luksChangeKey", "--key-slot", "0", device)

	currentPw, currentIsPassword := currentAuth.(*PasswordAuth)
	newPw, newIsPassword := newAuth.(*PasswordAuth)

	if currentIsPassword && newIsPassword {
		// Both passwords go to stdin. Write them both to a single pipe in sequence
		// so no intermediate userspace buffer is needed for either.
		pr, pw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		if _, err := pw.Write(currentPw.Password.Bytes()); err != nil {
			pr.Close()
			pw.Close()
			return fmt.Errorf("failed to write current password to pipe: %w", err)
		}
		pw.Write([]byte{'\n'})
		if _, err := pw.Write(newPw.Password.Bytes()); err != nil {
			pr.Close()
			pw.Close()
			return fmt.Errorf("failed to write new password to pipe: %w", err)
		}
		pw.Write([]byte{'\n'})
		pw.Close()
		cmd.Stdin = pr
		defer pr.Close()
	} else {
		// All other transitions: password→keyfile, keyfile→password, keyfile→keyfile
		if err := currentAuth.Apply(cmd); err != nil {
			return fmt.Errorf("failed to apply current authentication: %w", err)
		}
		defer closeStdinFile(cmd)
		if err := applyNewAuth(cmd, newAuth); err != nil {
			return fmt.Errorf("failed to apply new authentication: %w", err)
		}
	}

	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("cryptsetup luksChangeKey failed: %w", err)
	}

	return nil
}

// Dump retrieves metadata from a LUKS container header.
// No authentication is required - only reads public metadata.
func (m *LUKSManager) Dump(path string) (*LUKSDumpInfo, error) {
	output, err := m.executor.RunOutput("cryptsetup", "luksDump", path)
	if err != nil {
		return nil, fmt.Errorf("failed to read LUKS header: %w", err)
	}

	return parseLuksDump(string(output))
}

// parseLuksDump parses the output of cryptsetup luksDump
func parseLuksDump(output string) (*LUKSDumpInfo, error) {
	info := &LUKSDumpInfo{}
	lines := strings.Split(output, "\n")

	// Regular expressions for parsing
	versionRe := regexp.MustCompile(`^Version:\s+(\d+)`)
	uuidRe := regexp.MustCompile(`^UUID:\s+(\S+)`)
	cipherRe := regexp.MustCompile(`^Cipher name:\s+(\S+)`)
	cipherModeRe := regexp.MustCompile(`^Cipher mode:\s+(\S+)`)
	hashRe := regexp.MustCompile(`^Hash spec:\s+(\S+)`)
	// LUKS2 uses "Keyslots:" section, LUKS1 uses "Key Slot N: ENABLED"
	keySlotEnabledRe := regexp.MustCompile(`^Key Slot (\d+): ENABLED`)
	// LUKS2 keyslot format: "  N: luks2" at start of line in Keyslots section
	luks2KeySlotRe := regexp.MustCompile(`^\s+(\d+): luks2`)

	inKeyslots := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Check for LUKS2 Keyslots section
		if strings.HasPrefix(line, "Keyslots:") {
			inKeyslots = true
			continue
		}

		// End of Keyslots section in LUKS2 (next section starts)
		if inKeyslots && len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			inKeyslots = false
		}

		if matches := versionRe.FindStringSubmatch(line); matches != nil {
			info.Version = matches[1]
		} else if matches := uuidRe.FindStringSubmatch(line); matches != nil {
			info.UUID = matches[1]
		} else if matches := cipherRe.FindStringSubmatch(line); matches != nil {
			info.Cipher = matches[1]
		} else if matches := cipherModeRe.FindStringSubmatch(line); matches != nil {
			info.CipherMode = matches[1]
		} else if matches := hashRe.FindStringSubmatch(line); matches != nil {
			info.HashSpec = matches[1]
		} else if matches := keySlotEnabledRe.FindStringSubmatch(line); matches != nil {
			// LUKS1 format
			slot, _ := strconv.Atoi(matches[1])
			info.KeySlots = append(info.KeySlots, slot)
		} else if inKeyslots {
			if matches := luks2KeySlotRe.FindStringSubmatch(line); matches != nil {
				slot, _ := strconv.Atoi(matches[1])
				info.KeySlots = append(info.KeySlots, slot)
			}
		}
	}

	// Validate we got at least some data
	if info.Version == "" {
		return nil, fmt.Errorf("failed to parse LUKS header: version not found")
	}

	return info, nil
}

// TestPassphrase tests if authentication credentials are valid without opening the container.
func (m *LUKSManager) TestPassphrase(path string, auth AuthMethod) error {
	cmd := exec.Command("cryptsetup", "luksOpen", "--test-passphrase", path)
	if err := auth.Apply(cmd); err != nil {
		return err
	}
	defer closeStdinFile(cmd)

	_, err := m.executor.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf("passphrase verification failed: %w", err)
	}

	return nil
}

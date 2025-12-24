package system

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseSize converts size string (1G, 100M) to bytes
func ParseSize(s string) (uint64, error) {
	re := regexp.MustCompile(`^(\d+)([KMGT]?)$`)
	matches := re.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(s)))
	if matches == nil {
		return 0, fmt.Errorf("invalid size format: %s (use format like 1G, 100M, 500K)", s)
	}

	value, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size value: %s", matches[1])
	}

	unit := matches[2]
	multipliers := map[string]uint64{
		"":  1,
		"K": 1024,
		"M": 1024 * 1024,
		"G": 1024 * 1024 * 1024,
		"T": 1024 * 1024 * 1024 * 1024,
	}

	return value * multipliers[unit], nil
}

// FormatSize converts bytes to human-readable format
func FormatSize(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}

// ParseDmsetupTable extracts backing device from dmsetup table output
// Format: "0 sectors crypt cipher ... backing_device offset"
func ParseDmsetupTable(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) < 7 {
		return "", fmt.Errorf("invalid dmsetup table format")
	}
	// Backing device is typically the 7th field (index 6)
	return fields[6], nil
}

// ParseLosetupFind extracts loop device from losetup -j output
// Format: "/dev/loop0: [0042]:12345 (/path/to/file)"
func ParseLosetupFind(output string) (string, error) {
	parts := strings.SplitN(output, ":", 2)
	if len(parts) < 1 {
		return "", fmt.Errorf("invalid losetup output")
	}
	return strings.TrimSpace(parts[0]), nil
}

package container

import (
	"path/filepath"
	"regexp"
	"strings"
)

// GenerateMapperName converts a container path to a valid dm-crypt mapper name
// Mirrors the bash implementation:
// - /path/to/container.img → container_img
// - Replaces dots and dashes with underscores
// - Removes special characters
// - Prepends "crypt_" if starts with number
func GenerateMapperName(containerPath string) string {
	base := filepath.Base(containerPath)

	// Replace dots and dashes with underscores
	name := strings.ReplaceAll(base, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Remove non-alphanumeric except underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = re.ReplaceAllString(name, "")

	// Ensure doesn't start with number
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "crypt_" + name
	}

	return name
}

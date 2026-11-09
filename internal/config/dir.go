package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnvDir is the environment variable that overrides the config directory.
// Tests set it so they never touch the user's real config.
const EnvDir = "R_GIT_CONFIG_DIR"

// Dir returns the directory that holds application and per-repo settings.
// A non-empty R_GIT_CONFIG_DIR wins. Otherwise the directory is
// os.UserConfigDir()/r-git, which is %AppData%\r-git on Windows,
// ~/Library/Application Support/r-git on macOS, and $XDG_CONFIG_HOME/r-git
// (or ~/.config/r-git) on Linux.
func Dir() (string, error) {
	if override := os.Getenv(EnvDir); override != "" {
		return filepath.Clean(override), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "r-git"), nil
}

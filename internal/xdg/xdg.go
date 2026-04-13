package xdg

import (
	"os"
	"path/filepath"
)

// CacheHome returns $XDG_CACHE_HOME, defaulting to ~/.cache.
func CacheHome() (string, error) {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// StateHome returns $XDG_STATE_HOME, defaulting to ~/.local/state.
func StateHome() (string, error) {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}

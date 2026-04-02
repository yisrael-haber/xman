package config

import (
	"os"
	"path/filepath"
)

func appConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "xman"), nil
}

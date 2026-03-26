package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const keyringSvc = "xman"

// configPath returns the OS-appropriate path for the settings file.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "xman", "connection.json"), nil
}

// SaveConnection writes a ConnectRequest to disk.
// For vCenter, the password is stored in the OS keyring rather than the file.
func SaveConnection(req ConnectRequest) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// Strip the password before writing to disk
	safe := req
	safe.Password = ""
	data, err := json.MarshalIndent(safe, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	// For vCenter, store the password in the OS keyring keyed by username.
	if req.BackendType == "vcenter" && req.Password != "" && req.Username != "" {
		_ = keyring.Set(keyringSvc, req.Username, req.Password)
	}
	return nil
}

// LoadConnection reads the saved ConnectRequest from disk and restores the
// vCenter password from the OS keyring if available.
// Returns an empty ConnectRequest (no error) if nothing has been saved yet.
func LoadConnection() (ConnectRequest, error) {
	path, err := configPath()
	if err != nil {
		return ConnectRequest{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConnectRequest{}, nil
		}
		return ConnectRequest{}, err
	}

	var req ConnectRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return ConnectRequest{}, err
	}

	// Migrate settings saved before BackendType was introduced
	if req.BackendType == "" && req.URL != "" {
		req.BackendType = "vcenter"
	}

	// Restore vCenter password from keyring (best-effort)
	if req.BackendType == "vcenter" && req.Username != "" {
		if pw, err := keyring.Get(keyringSvc, req.Username); err == nil {
			req.Password = pw
		}
	}

	return req, nil
}

// ClearConnection removes the saved settings file and the keyring entry.
func ClearConnection() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	// Load first to get the username for keyring cleanup
	saved, _ := LoadConnection()
	if saved.BackendType == "vcenter" && saved.Username != "" {
		_ = keyring.Delete(keyringSvc, saved.Username)
	}

	_ = os.Remove(path)
	return nil
}

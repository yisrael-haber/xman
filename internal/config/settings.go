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

// savedSettings is the on-disk format.  Each backend has its own section so
// that saving one never overwrites the other.
type savedSettings struct {
	LastBackend string          `json:"lastBackend"`
	VCenter     vcenterSettings `json:"vcenter"`
	Workstation wsSettings      `json:"workstation"`
}

type vcenterSettings struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Insecure bool   `json:"insecure"`
}

type wsSettings struct {
	VmrunPath string `json:"vmrunPath"`
	VMDir     string `json:"vmDir"`
}

// loadFile reads and parses the settings file, handling both the current
// format and the legacy flat ConnectRequest format.
func loadFile(path string) (savedSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return savedSettings{}, nil
		}
		return savedSettings{}, err
	}

	var s savedSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return savedSettings{}, err
	}

	// Migrate from the old flat ConnectRequest format (no lastBackend key).
	if s.LastBackend == "" {
		var old ConnectRequest
		if err := json.Unmarshal(data, &old); err == nil && old.BackendType != "" {
			s.LastBackend = old.BackendType
			if old.BackendType == "vcenter" {
				s.VCenter = vcenterSettings{URL: old.URL, Username: old.Username, Insecure: old.Insecure}
			} else if old.BackendType == "workstation" {
				s.Workstation = wsSettings{VmrunPath: old.VmrunPath, VMDir: old.VMDir}
			}
		}
	}

	return s, nil
}

// SaveConnection writes only the section for req.BackendType, leaving the
// other backend's settings untouched.
func SaveConnection(req ConnectRequest) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// Read current file so we can preserve the other backend's section.
	s, _ := loadFile(path)
	s.LastBackend = req.BackendType

	switch req.BackendType {
	case "vcenter":
		s.VCenter = vcenterSettings{URL: req.URL, Username: req.Username, Insecure: req.Insecure}
		if req.Password != "" && req.Username != "" {
			_ = keyring.Set(keyringSvc, req.Username, req.Password)
		}
	case "workstation":
		s.Workstation = wsSettings{VmrunPath: req.VmrunPath, VMDir: req.VMDir}
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadConnection reads both backends' saved settings and returns them merged
// into a single ConnectRequest.  BackendType is set to the last-used backend.
// The vCenter password is restored from the OS keyring if available.
// Returns an empty ConnectRequest (no error) if nothing has been saved yet.
func LoadConnection() (ConnectRequest, error) {
	path, err := configPath()
	if err != nil {
		return ConnectRequest{}, err
	}

	s, err := loadFile(path)
	if err != nil {
		return ConnectRequest{}, err
	}

	req := ConnectRequest{
		BackendType: s.LastBackend,
		URL:         s.VCenter.URL,
		Username:    s.VCenter.Username,
		Insecure:    s.VCenter.Insecure,
		VmrunPath:   s.Workstation.VmrunPath,
		VMDir:       s.Workstation.VMDir,
	}

	// Restore vCenter password from keyring (best-effort).
	if req.Username != "" {
		if pw, err := keyring.Get(keyringSvc, req.Username); err == nil {
			req.Password = pw
		}
	}

	return req, nil
}

// ClearConnection removes the saved settings file and any keyring entry.
func ClearConnection() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	saved, _ := LoadConnection()
	if saved.Username != "" {
		_ = keyring.Delete(keyringSvc, saved.Username)
	}

	_ = os.Remove(path)
	return nil
}

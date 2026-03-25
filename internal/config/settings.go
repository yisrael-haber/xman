package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const keyringSvc = "manosphere"

// ConnectionSettings holds all fields needed to reconnect.
// Password is loaded from the OS keyring, never written to disk.
type ConnectionSettings struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Insecure bool   `json:"insecure"`
	Password string `json:"password,omitempty"` // populated from keyring, not the JSON file
}

// configPath returns the OS-appropriate path for the settings file:
//   - Linux/WSL2: ~/.config/manosphere/connection.json
//   - Windows:    %AppData%\manosphere\connection.json
//   - macOS:      ~/Library/Application Support/manosphere/connection.json
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "manosphere", "connection.json"), nil
}

// SaveConnection writes URL/username/insecure to disk and the password to the OS keyring.
func SaveConnection(s ConnectionSettings) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// Write non-sensitive fields to JSON — omit password field entirely
	data, err := json.MarshalIndent(struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Insecure bool   `json:"insecure"`
	}{s.URL, s.Username, s.Insecure}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	// Best-effort: store password in the OS keyring.
	// Silently ignore keyring errors (e.g. no daemon running in WSL2) —
	// the user will just need to re-enter their password next time.
	if s.Password != "" {
		_ = keyring.Set(keyringSvc, s.Username, s.Password)
	}
	return nil
}

// LoadConnection reads the JSON settings and fetches the password from the keyring.
// Returns empty defaults (no error) if no settings have been saved yet.
func LoadConnection() (ConnectionSettings, error) {
	path, err := configPath()
	if err != nil {
		return ConnectionSettings{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConnectionSettings{}, nil
		}
		return ConnectionSettings{}, err
	}

	var s ConnectionSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return ConnectionSettings{}, err
	}

	// Best-effort: fetch password from keyring — silently ignore if not available
	if s.Username != "" {
		if pw, err := keyring.Get(keyringSvc, s.Username); err == nil {
			s.Password = pw
		}
	}

	return s, nil
}

// DeleteConnection removes saved settings from disk and the password from the keyring.
func DeleteConnection(username string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	_ = keyring.Delete(keyringSvc, username)
	return nil
}

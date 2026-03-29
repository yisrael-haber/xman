package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const guestCredentialKeyringSvc = "xman-guest-credentials"

// GuestCredentialMeta describes a stored guest-operations username/password pair.
type GuestCredentialMeta struct {
	Label    string `json:"label"`
	Username string `json:"username"`
}

// GuestCredential is the full in-memory credential including the secret password.
type GuestCredential struct {
	GuestCredentialMeta
	Password string `json:"password"`
}

func guestCredentialsDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "xman", "guest-credentials"), nil
}

func guestCredentialLabelDir(label string) (string, error) {
	base, err := guestCredentialsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, label), nil
}

func normalizeCredentialLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", errors.New("label is required")
	}
	if strings.ContainsAny(label, `/\`+"\x00") || label == "." || label == ".." {
		return "", errors.New("label contains invalid characters")
	}
	return label, nil
}

func normalizeGuestCredentialInput(label, username, password string) (string, string, string, error) {
	var err error
	label, err = normalizeCredentialLabel(label)
	if err != nil {
		return "", "", "", err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return "", "", "", errors.New("username is required")
	}
	if password == "" {
		return "", "", "", errors.New("password is required")
	}

	return label, username, password, nil
}

// CreateGuestCredential stores the credential metadata on disk and the password in the OS keyring.
func CreateGuestCredential(label, username, password string) (GuestCredentialMeta, error) {
	var err error
	label, username, password, err = normalizeGuestCredentialInput(label, username, password)
	if err != nil {
		return GuestCredentialMeta{}, err
	}

	dir, err := guestCredentialLabelDir(label)
	if err != nil {
		return GuestCredentialMeta{}, err
	}
	if _, err := os.Stat(dir); err == nil {
		return GuestCredentialMeta{}, fmt.Errorf("credential %q already exists", label)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return GuestCredentialMeta{}, err
	}

	meta := GuestCredentialMeta{
		Label:    label,
		Username: username,
	}

	if err := keyring.Set(guestCredentialKeyringSvc, label, password); err != nil {
		_ = os.RemoveAll(dir)
		return GuestCredentialMeta{}, fmt.Errorf("storing password for %q: %w", label, err)
	}

	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		_ = keyring.Delete(guestCredentialKeyringSvc, label)
		_ = os.RemoveAll(dir)
		return GuestCredentialMeta{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaJSON, 0o600); err != nil {
		_ = keyring.Delete(guestCredentialKeyringSvc, label)
		_ = os.RemoveAll(dir)
		return GuestCredentialMeta{}, err
	}

	return meta, nil
}

// ListGuestCredentials returns metadata for all stored guest credentials.
func ListGuestCredentials() ([]GuestCredentialMeta, error) {
	base, err := guestCredentialsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return []GuestCredentialMeta{}, nil
	}
	if err != nil {
		return nil, err
	}

	var creds []GuestCredentialMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(base, entry.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var meta GuestCredentialMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		creds = append(creds, meta)
	}
	if creds == nil {
		creds = []GuestCredentialMeta{}
	}
	return creds, nil
}

// GetGuestCredential returns metadata for a single stored guest credential.
func GetGuestCredential(label string) (GuestCredentialMeta, error) {
	label, err := normalizeCredentialLabel(label)
	if err != nil {
		return GuestCredentialMeta{}, err
	}

	dir, err := guestCredentialLabelDir(label)
	if err != nil {
		return GuestCredentialMeta{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return GuestCredentialMeta{}, fmt.Errorf("credential %q not found", label)
	}

	var meta GuestCredentialMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return GuestCredentialMeta{}, err
	}
	return meta, nil
}

// LoadGuestCredential returns the full credential including the password from the OS keyring.
func LoadGuestCredential(label string) (GuestCredential, error) {
	meta, err := GetGuestCredential(label)
	if err != nil {
		return GuestCredential{}, err
	}

	password, err := keyring.Get(guestCredentialKeyringSvc, meta.Label)
	if err != nil {
		return GuestCredential{}, fmt.Errorf("loading password for %q: %w", meta.Label, err)
	}

	return GuestCredential{
		GuestCredentialMeta: meta,
		Password:            password,
	}, nil
}

// UpdateGuestCredential rewrites an existing credential, optionally renaming its label.
func UpdateGuestCredential(currentLabel, newLabel, username, password string) (GuestCredentialMeta, error) {
	currentLabel, err := normalizeCredentialLabel(currentLabel)
	if err != nil {
		return GuestCredentialMeta{}, err
	}
	newLabel, username, password, err = normalizeGuestCredentialInput(newLabel, username, password)
	if err != nil {
		return GuestCredentialMeta{}, err
	}

	currentDir, err := guestCredentialLabelDir(currentLabel)
	if err != nil {
		return GuestCredentialMeta{}, err
	}
	if _, err := os.Stat(currentDir); errors.Is(err, os.ErrNotExist) {
		return GuestCredentialMeta{}, fmt.Errorf("credential %q not found", currentLabel)
	}
	oldMeta, err := GetGuestCredential(currentLabel)
	if err != nil {
		return GuestCredentialMeta{}, err
	}
	oldMetaJSON, err := json.MarshalIndent(oldMeta, "", "  ")
	if err != nil {
		return GuestCredentialMeta{}, err
	}

	targetDir, err := guestCredentialLabelDir(newLabel)
	if err != nil {
		return GuestCredentialMeta{}, err
	}
	if currentLabel != newLabel {
		if _, err := os.Stat(targetDir); err == nil {
			return GuestCredentialMeta{}, fmt.Errorf("credential %q already exists", newLabel)
		}
	}

	meta := GuestCredentialMeta{
		Label:    newLabel,
		Username: username,
	}
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return GuestCredentialMeta{}, err
	}

	if currentLabel != newLabel {
		if err := os.Rename(currentDir, targetDir); err != nil {
			return GuestCredentialMeta{}, fmt.Errorf("renaming credential %q to %q: %w", currentLabel, newLabel, err)
		}
	} else {
		targetDir = currentDir
	}

	rollbackDir := currentDir
	if currentLabel == newLabel {
		rollbackDir = targetDir
	}

	if err := os.WriteFile(filepath.Join(targetDir, "meta.json"), metaJSON, 0o600); err != nil {
		if currentLabel != newLabel {
			_ = os.Rename(targetDir, rollbackDir)
		}
		return GuestCredentialMeta{}, err
	}

	if err := keyring.Set(guestCredentialKeyringSvc, newLabel, password); err != nil {
		if currentLabel != newLabel {
			_ = os.Rename(targetDir, rollbackDir)
			_ = os.WriteFile(filepath.Join(rollbackDir, "meta.json"), oldMetaJSON, 0o600)
		} else {
			_ = os.WriteFile(filepath.Join(targetDir, "meta.json"), oldMetaJSON, 0o600)
		}
		return GuestCredentialMeta{}, fmt.Errorf("storing password for %q: %w", newLabel, err)
	}
	if currentLabel != newLabel {
		_ = keyring.Delete(guestCredentialKeyringSvc, currentLabel)
	}

	return meta, nil
}

// DeleteGuestCredential removes the credential metadata and best-effort deletes the password from the keyring.
func DeleteGuestCredential(label string) error {
	label, err := normalizeCredentialLabel(label)
	if err != nil {
		return err
	}

	dir, err := guestCredentialLabelDir(label)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("credential %q not found", label)
	}

	_ = keyring.Delete(guestCredentialKeyringSvc, label)
	return os.RemoveAll(dir)
}

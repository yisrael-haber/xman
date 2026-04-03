package manager

import (
	"strings"

	"xman/internal/config"
)

var loadGuestCredential = config.LoadGuestCredential

func resolveGuestCredential(label, username, password string) (string, string, error) {
	if strings.TrimSpace(label) == "" {
		return username, password, nil
	}

	cred, err := loadGuestCredential(strings.TrimSpace(label))
	if err != nil {
		return "", "", err
	}
	return cred.Username, cred.Password, nil
}

func resolveRequestCredential(label string, username, password *string) error {
	resolvedUsername, resolvedPassword, err := resolveGuestCredential(label, *username, *password)
	if err != nil {
		return err
	}
	*username = resolvedUsername
	*password = resolvedPassword
	return nil
}

func resolveRunRequestCredential(req RunRequest) (RunRequest, error) {
	if err := resolveRequestCredential(req.CredentialLabel, &req.Username, &req.Password); err != nil {
		return RunRequest{}, err
	}
	return req, nil
}

func resolveUploadRequestCredential(req UploadRequest) (UploadRequest, error) {
	if err := resolveRequestCredential(req.CredentialLabel, &req.Username, &req.Password); err != nil {
		return UploadRequest{}, err
	}
	return req, nil
}

func resolveDownloadRequestCredential(req DownloadRequest) (DownloadRequest, error) {
	if err := resolveRequestCredential(req.CredentialLabel, &req.Username, &req.Password); err != nil {
		return DownloadRequest{}, err
	}
	return req, nil
}

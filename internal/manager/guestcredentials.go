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

func resolveRunRequestCredential(req RunRequest) (RunRequest, error) {
	username, password, err := resolveGuestCredential(req.CredentialLabel, req.Username, req.Password)
	if err != nil {
		return RunRequest{}, err
	}
	req.Username = username
	req.Password = password
	return req, nil
}

func resolveUploadRequestCredential(req UploadRequest) (UploadRequest, error) {
	username, password, err := resolveGuestCredential(req.CredentialLabel, req.Username, req.Password)
	if err != nil {
		return UploadRequest{}, err
	}
	req.Username = username
	req.Password = password
	return req, nil
}

func resolveDownloadRequestCredential(req DownloadRequest) (DownloadRequest, error) {
	username, password, err := resolveGuestCredential(req.CredentialLabel, req.Username, req.Password)
	if err != nil {
		return DownloadRequest{}, err
	}
	req.Username = username
	req.Password = password
	return req, nil
}

func resolveInstallRequestCredential(req InstallRequest) (InstallRequest, error) {
	username, password, err := resolveGuestCredential(req.CredentialLabel, req.Username, req.Password)
	if err != nil {
		return InstallRequest{}, err
	}
	req.Username = username
	req.Password = password
	return req, nil
}

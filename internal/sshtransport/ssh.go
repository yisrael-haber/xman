package sshtransport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"xman/internal/config"
	"xman/internal/jobs"
)

const defaultSSHPort = 22

func sshClientConfig(username string, auth ssh.AuthMethod) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec -- internal VM management
		Timeout:         15 * time.Second,
	}
}

func dialWithPassword(host string, port int, username, password string) (*ssh.Client, string, error) {
	target, err := ParseHost(host, port)
	if err != nil {
		return nil, "", err
	}
	client, err := ssh.Dial("tcp", target.Address, sshClientConfig(username, ssh.Password(password)))
	return client, target.Display, err
}

func dialWithKey(host, keyLabel string) (*ssh.Client, config.KeyMeta, string, error) {
	meta, signer, err := config.LoadKeySigner(keyLabel)
	if err != nil {
		return nil, config.KeyMeta{}, "", err
	}
	if strings.TrimSpace(meta.DefaultUser) == "" {
		return nil, config.KeyMeta{}, "", fmt.Errorf("key %q has no default user; set one in SSH Keys before using it for SSH/SFTP", keyLabel)
	}

	target, err := ParseHost(host, defaultSSHPort)
	if err != nil {
		return nil, config.KeyMeta{}, "", err
	}

	client, err := ssh.Dial("tcp", target.Address, sshClientConfig(meta.DefaultUser, ssh.PublicKeys(signer)))
	return client, meta, target.Display, err
}

// cancelOnContext closes client when ctx is cancelled, and stops when done is closed.
func cancelOnContext(ctx context.Context, client *ssh.Client, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		client.Close()
	case <-done:
	}
}

func withKeySession(ctx context.Context, emit jobs.EmitFn, host, keyLabel string, fn func(session *KeySession) error) error {
	session, err := DialKey(ctx, emit, host, keyLabel)
	if err != nil {
		return err
	}
	defer session.Close()
	return fn(session)
}

func Run(ctx context.Context, emit jobs.EmitFn, host, keyLabel, command string) error {
	return withKeySession(ctx, emit, host, keyLabel, func(session *KeySession) error {
		return session.Run(emit, command)
	})
}

func Upload(ctx context.Context, emit jobs.EmitFn, host, keyLabel, localPath, remotePath string) error {
	return withKeySession(ctx, emit, host, keyLabel, func(session *KeySession) error {
		return session.Upload(emit, localPath, remotePath)
	})
}

func Download(ctx context.Context, emit jobs.EmitFn, host, keyLabel, remotePath, localPath string) error {
	return withKeySession(ctx, emit, host, keyLabel, func(session *KeySession) error {
		return session.Download(emit, remotePath, localPath)
	})
}

func Remove(ctx context.Context, host, keyLabel, remotePath string) error {
	return withKeySession(ctx, nil, host, keyLabel, func(session *KeySession) error {
		return session.Remove(remotePath)
	})
}

func VerifyKey(ctx context.Context, host, keyLabel string) error {
	return withKeySession(ctx, nil, host, keyLabel, func(session *KeySession) error {
		return session.Verify()
	})
}

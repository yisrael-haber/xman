package sshtransport

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"xman/internal/config"
	"xman/internal/executil"
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

func dialAddress(host string, defaultPort int) (string, string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", "", fmt.Errorf("host is required")
	}

	if _, _, err := net.SplitHostPort(host); err == nil {
		return host, host, nil
	}

	display := net.JoinHostPort(host, strconv.Itoa(defaultPort))
	return display, display, nil
}

func dialWithPassword(host string, port int, username, password string) (*ssh.Client, string, error) {
	if port <= 0 {
		port = defaultSSHPort
	}
	addr := net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port))
	client, err := ssh.Dial("tcp", addr, sshClientConfig(username, ssh.Password(password)))
	return client, addr, err
}

func dialWithKey(host, keyLabel string) (*ssh.Client, config.KeyMeta, string, error) {
	meta, signer, err := config.LoadKeySigner(keyLabel)
	if err != nil {
		return nil, config.KeyMeta{}, "", err
	}
	if strings.TrimSpace(meta.DefaultUser) == "" {
		return nil, config.KeyMeta{}, "", fmt.Errorf("key %q has no default user; set one in SSH Keys before using it for SSH/SFTP", keyLabel)
	}

	addr, display, err := dialAddress(host, defaultSSHPort)
	if err != nil {
		return nil, config.KeyMeta{}, "", err
	}

	client, err := ssh.Dial("tcp", addr, sshClientConfig(meta.DefaultUser, ssh.PublicKeys(signer)))
	return client, meta, display, err
}

// cancelOnContext closes client when ctx is cancelled, and stops when done is closed.
func cancelOnContext(ctx context.Context, client *ssh.Client, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		client.Close()
	case <-done:
	}
}

func withKeyClient(ctx context.Context, emit jobs.EmitFn, host, keyLabel string, fn func(client *ssh.Client, meta config.KeyMeta, display string) error) error {
	client, meta, display, err := dialWithKey(host, keyLabel)
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()

	if emit != nil {
		emit(5, fmt.Sprintf("Connecting to %s as %s with key %s...", display, meta.DefaultUser, meta.Label))
	}

	done := make(chan struct{})
	defer close(done)
	go cancelOnContext(ctx, client, done)

	return fn(client, meta, display)
}

func Run(ctx context.Context, emit jobs.EmitFn, host, keyLabel, command string) error {
	return withKeyClient(ctx, emit, host, keyLabel, func(client *ssh.Client, _ config.KeyMeta, _ string) error {
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("SSH session: %w", err)
		}
		defer session.Close()

		emit(20, "Running command...")
		raw, err := session.CombinedOutput(command)

		output := executil.NormalizeCapturedOutput(raw)
		if err != nil {
			emit(95, output+"\n\n["+err.Error()+"]")
			emit(100, "Command finished with non-zero exit status.")
			return nil
		}
		emit(95, output)
		emit(100, "Command completed.")
		return nil
	})
}

func Upload(ctx context.Context, emit jobs.EmitFn, host, keyLabel, localPath, remotePath string) error {
	return withKeyClient(ctx, emit, host, keyLabel, func(client *ssh.Client, _ config.KeyMeta, _ string) error {
		sc, err := sftp.NewClient(client)
		if err != nil {
			return fmt.Errorf("SFTP client: %w", err)
		}
		defer sc.Close()

		emit(10, fmt.Sprintf("Uploading %s...", localPath))

		src, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("opening local file: %w", err)
		}
		defer src.Close()

		dst, err := sc.Create(remotePath)
		if err != nil {
			return fmt.Errorf("creating remote file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("uploading: %w", err)
		}
		emit(100, "Upload complete.")
		return nil
	})
}

func Download(ctx context.Context, emit jobs.EmitFn, host, keyLabel, remotePath, localPath string) error {
	return withKeyClient(ctx, emit, host, keyLabel, func(client *ssh.Client, _ config.KeyMeta, _ string) error {
		sc, err := sftp.NewClient(client)
		if err != nil {
			return fmt.Errorf("SFTP client: %w", err)
		}
		defer sc.Close()

		emit(10, fmt.Sprintf("Downloading %s...", remotePath))

		src, err := sc.Open(remotePath)
		if err != nil {
			return fmt.Errorf("opening remote file: %w", err)
		}
		defer src.Close()

		dst, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("creating local file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("downloading: %w", err)
		}
		emit(100, "Download complete.")
		return nil
	})
}

func VerifyKey(ctx context.Context, host, keyLabel string) error {
	return withKeyClient(ctx, nil, host, keyLabel, func(client *ssh.Client, _ config.KeyMeta, _ string) error {
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("SSH session: %w", err)
		}
		session.Close()
		return nil
	})
}

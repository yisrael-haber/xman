package sshtransport

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"xman/internal/jobs"
)

func dial(host string, port int, username, password string) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec — internal VM management
		Timeout:         15 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), cfg)
}

func Run(ctx context.Context, emit jobs.EmitFn, host string, port int, username, password, command string) error {
	emit(5, fmt.Sprintf("Connecting to %s:%d...", host, port))
	client, err := dial(host, port, username, password)
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()

	// Close the connection if the job is cancelled.
	go func() {
		<-ctx.Done()
		client.Close()
	}()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session: %w", err)
	}
	defer session.Close()

	emit(20, "Running command...")
	raw, err := session.CombinedOutput(command)

	output := strings.TrimSpace(string(raw))
	if len(output) > 16*1024 {
		output = output[:16*1024] + "\n[output truncated]"
	}
	if output == "" {
		output = "(no output)"
	}

	// Non-zero exit — show output anyway with the exit error appended.
	if err != nil {
		emit(100, output+"\n\n["+err.Error()+"]")
		return nil
	}
	emit(100, output)
	return nil
}

func Upload(ctx context.Context, emit jobs.EmitFn, host string, port int, username, password, localPath, remotePath string) error {
	emit(5, fmt.Sprintf("Connecting to %s:%d...", host, port))
	client, err := dial(host, port, username, password)
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()

	go func() { <-ctx.Done(); client.Close() }()

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
}

func Download(ctx context.Context, emit jobs.EmitFn, host string, port int, username, password, remotePath, localPath string) error {
	emit(5, fmt.Sprintf("Connecting to %s:%d...", host, port))
	client, err := dial(host, port, username, password)
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer client.Close()

	go func() { <-ctx.Done(); client.Close() }()

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
}

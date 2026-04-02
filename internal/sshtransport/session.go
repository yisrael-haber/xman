package sshtransport

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"xman/internal/config"
	"xman/internal/executil"
	"xman/internal/jobs"
)

const sftpMaxConcurrentRequestsPerFile = 64

func newSFTPClient(client *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(client,
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(sftpMaxConcurrentRequestsPerFile),
	)
}

// KeySession keeps a single SSH connection alive across multiple operations.
type KeySession struct {
	client  *ssh.Client
	meta    config.KeyMeta
	display string

	mu   sync.Mutex
	sftp *sftp.Client

	done      chan struct{}
	closeOnce sync.Once
}

func DialKey(ctx context.Context, emit jobs.EmitFn, host, keyLabel string) (*KeySession, error) {
	client, meta, display, err := dialWithKey(host, keyLabel)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}

	session := &KeySession{
		client:  client,
		meta:    meta,
		display: display,
		done:    make(chan struct{}),
	}

	if emit != nil {
		emit(5, fmt.Sprintf("Connecting to %s as %s with key %s...", display, meta.DefaultUser, meta.Label))
	}

	go cancelOnContext(ctx, client, session.done)
	return session, nil
}

func (s *KeySession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		close(s.done)

		s.mu.Lock()
		sc := s.sftp
		s.sftp = nil
		s.mu.Unlock()
		if sc != nil {
			closeErr = sc.Close()
		}
		if err := s.client.Close(); closeErr == nil && err != nil {
			closeErr = err
		}
	})
	return closeErr
}

func (s *KeySession) sftpClient() (*sftp.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sftp != nil {
		return s.sftp, nil
	}

	sc, err := newSFTPClient(s.client)
	if err != nil {
		return nil, fmt.Errorf("SFTP client: %w", err)
	}
	s.sftp = sc
	return s.sftp, nil
}

func (s *KeySession) Run(emit jobs.EmitFn, command string) error {
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session: %w", err)
	}
	defer session.Close()

	if emit != nil {
		emit(20, "Running command...")
	}

	raw, err := session.CombinedOutput(command)
	output := executil.NormalizeCapturedOutput(raw)
	if err != nil {
		if emit != nil {
			emit(95, output+"\n\n["+err.Error()+"]")
			emit(100, "Command finished with non-zero exit status.")
		}
		return nil
	}

	if emit != nil {
		emit(95, output)
		emit(100, "Command completed.")
	}
	return nil
}

func (s *KeySession) Upload(emit jobs.EmitFn, localPath, remotePath string) error {
	sc, err := s.sftpClient()
	if err != nil {
		return err
	}

	if emit != nil {
		emit(10, fmt.Sprintf("Uploading %s...", localPath))
	}

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

	written, err := dst.ReadFrom(src)
	if err != nil {
		_ = dst.Truncate(written)
		return fmt.Errorf("uploading: %w", err)
	}
	if emit != nil {
		emit(100, "Upload complete.")
	}
	return nil
}

func (s *KeySession) Download(emit jobs.EmitFn, remotePath, localPath string) error {
	sc, err := s.sftpClient()
	if err != nil {
		return err
	}

	if emit != nil {
		emit(10, fmt.Sprintf("Downloading %s...", remotePath))
	}

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

	if _, err := src.WriteTo(dst); err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	if emit != nil {
		emit(100, "Download complete.")
	}
	return nil
}

func (s *KeySession) Remove(remotePath string) error {
	sc, err := s.sftpClient()
	if err != nil {
		return err
	}
	if err := sc.Remove(remotePath); err != nil {
		return fmt.Errorf("removing remote file: %w", err)
	}
	return nil
}

func (s *KeySession) Verify() error {
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session: %w", err)
	}
	return session.Close()
}

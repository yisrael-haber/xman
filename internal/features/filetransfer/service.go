package filetransfer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"
)

// GuestAuth holds guest OS credentials for a VM.
type GuestAuth struct {
	Username string
	Password string
}

// Service provides file transfer operations to/from VM guests.
type Service struct {
	session *vcenter.Session
}

// NewService creates a filetransfer Service.
func NewService(s *vcenter.Session) *Service {
	return &Service{session: s}
}

// Upload copies a local file into a VM guest filesystem.
// Progress is reported via emit (0-100).
func (s *Service) Upload(ctx context.Context, emit jobs.EmitFn, vmRef string, auth GuestAuth, localPath, guestPath string) error {
	client, err := s.session.Client()
	if err != nil {
		return err
	}

	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmRef}
	ops := guest.NewOperationsManager(client.Client, ref)

	fm, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest file manager: %w", err)
	}

	guestAuth := &types.NamePasswordAuthentication{
		Username: auth.Username,
		Password: auth.Password,
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening local file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	emit(10, "Initiating transfer to guest...")

	rawURL, err := fm.InitiateFileTransferToGuest(ctx, guestAuth, guestPath,
		&types.GuestPosixFileAttributes{}, fi.Size(), true)
	if err != nil {
		return fmt.Errorf("initiating upload: %w", err)
	}

	transferURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing transfer URL: %w", err)
	}

	emit(20, fmt.Sprintf("Uploading %s...", filepath.Base(localPath)))

	uploadParams := soap.DefaultUpload
	uploadParams.ContentLength = fi.Size()

	if err := client.Client.Upload(ctx, f, transferURL, &uploadParams); err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}

	emit(100, "Upload complete.")
	return nil
}

// Download copies a file from a VM guest filesystem to a local path.
// Progress is reported via emit (0-100).
func (s *Service) Download(ctx context.Context, emit jobs.EmitFn, vmRef string, auth GuestAuth, guestPath, localPath string) error {
	client, err := s.session.Client()
	if err != nil {
		return err
	}

	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmRef}
	ops := guest.NewOperationsManager(client.Client, ref)

	fm, err := ops.FileManager(ctx)
	if err != nil {
		return fmt.Errorf("getting guest file manager: %w", err)
	}

	guestAuth := &types.NamePasswordAuthentication{
		Username: auth.Username,
		Password: auth.Password,
	}

	emit(10, "Requesting file info from guest...")

	fileInfo, err := fm.InitiateFileTransferFromGuest(ctx, guestAuth, guestPath)
	if err != nil {
		return fmt.Errorf("initiating download: %w", err)
	}

	transferURL, err := url.Parse(fileInfo.Url)
	if err != nil {
		return fmt.Errorf("parsing transfer URL: %w", err)
	}

	emit(20, fmt.Sprintf("Downloading %s...", filepath.Base(guestPath)))

	if err := client.Client.DownloadFile(ctx, localPath, transferURL, nil); err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	emit(100, "Download complete.")
	return nil
}

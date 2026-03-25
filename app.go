package main

import (
	"context"

	"manosphere/internal/config"
	"manosphere/internal/features/filetransfer"
	"manosphere/internal/features/guestexec"
	"manosphere/internal/features/inventory"
	"manosphere/internal/features/packetcapture"
	"manosphere/internal/features/snapshots"
	"manosphere/internal/features/vminfo"
	"manosphere/internal/jobs"
	"manosphere/internal/vcenter"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the top-level Wails application struct.
// It wires together the vCenter session, job manager, and feature bindings.
// Business logic lives in the feature packages, not here.
type App struct {
	ctx     context.Context
	session *vcenter.Session

	// Feature bindings — each is registered with Wails and auto-generates
	// TypeScript bindings for the frontend.
	VMInfo        *vminfo.Binding
	FileTransfer  *filetransfer.Binding
	PacketCapture *packetcapture.Binding
	Snapshots     *snapshots.Binding
	GuestExec     *guestexec.Binding
	Inventory     *inventory.Binding
	Jobs          *jobs.Manager
}

// NewApp constructs the App and wires all features to the shared session.
func NewApp() *App {
	session := &vcenter.Session{}
	jobManager := &jobs.Manager{} // context injected in startup

	return &App{
		session:       session,
		VMInfo:        vminfo.NewBinding(session),
		FileTransfer:  filetransfer.NewBinding(session, jobManager),
		PacketCapture: packetcapture.NewBinding(session, jobManager),
		Snapshots:     snapshots.NewBinding(session, jobManager),
		GuestExec:     guestexec.NewBinding(session, jobManager),
		Inventory:     inventory.NewBinding(session),
		Jobs:          jobManager,
	}
}

// startup is called by Wails after the frontend is ready.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	*a.Jobs = *jobs.NewManager(ctx)
	a.VMInfo.SetContext(ctx)
	a.FileTransfer.SetContext(ctx)
	a.PacketCapture.SetContext(ctx)
	a.Snapshots.SetContext(ctx)
	a.GuestExec.SetContext(ctx)
	a.Inventory.SetContext(ctx)
}

// shutdown is called by Wails when the application is closing.
func (a *App) shutdown(ctx context.Context) {
	_ = a.session.Disconnect(ctx)
}

// --- vCenter connection bindings ---

func (a *App) Connect(url, username, password string, insecure bool) error {
	return a.session.Connect(context.Background(), vcenter.ConnectParams{
		URL:      url,
		Username: username,
		Password: password,
		Insecure: insecure,
	})
}

func (a *App) Disconnect() error {
	return a.session.Disconnect(context.Background())
}

func (a *App) ConnectionStatus() string {
	if !a.session.IsConnected() {
		return ""
	}
	return a.session.Host()
}

// --- Settings bindings ---

func (a *App) LoadConnectionSettings() config.ConnectionSettings {
	s, _ := config.LoadConnection()
	return s
}

func (a *App) SaveConnectionSettings(url, username, password string, insecure bool) error {
	return config.SaveConnection(config.ConnectionSettings{
		URL:      url,
		Username: username,
		Password: password,
		Insecure: insecure,
	})
}

func (a *App) ClearConnectionSettings(username string) error {
	return config.DeleteConnection(username)
}

// --- File dialog bindings ---

// OpenFileDialog opens a native file picker and returns the selected path.
func (a *App) OpenFileDialog(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
}

// SaveFileDialog opens a native save dialog and returns the chosen path.
func (a *App) SaveFileDialog(title, defaultFilename string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultFilename,
	})
}

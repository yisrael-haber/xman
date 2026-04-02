package main

import (
	"context"
	"fmt"

	"xman/internal/config"
	"xman/internal/jobs"
	"xman/internal/manager"
	"xman/internal/vcenter"
	"xman/internal/workstation"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App wires together the Manager, job system, and connection lifecycle.
// All VM/feature operations live on Manager; connection, settings, and
// file dialogs live here.
type App struct {
	ctx     context.Context
	Manager *manager.Manager
	Jobs    *jobs.Manager
}

// NewApp constructs the App. The backend is nil until Connect is called.
func NewApp() *App {
	jobManager := jobs.NewManager(nil)
	return &App{
		Manager: manager.New(jobManager),
		Jobs:    jobManager,
	}
}

// startup is called by Wails after the frontend is ready.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.Jobs.SetContext(ctx)
	a.Manager.SetContext(ctx)
}

// shutdown is called by Wails when the application is closing.
func (a *App) shutdown(ctx context.Context) {
	_ = a.Manager.Disconnect(ctx)
}

// --- Connection ---

// Connect creates the appropriate backend based on req.BackendType and
// installs it in the Manager. Returns ConnectionInfo on success.
func (a *App) Connect(req config.ConnectRequest) (config.ConnectionInfo, error) {
	var b manager.Backend
	var err error

	switch req.BackendType {
	case "vcenter":
		b, err = vcenter.NewBackend(a.ctx, req.URL, req.Username, req.Password, req.Insecure)
	case "workstation":
		b, err = workstation.NewBackend(req.VmrunPath, req.VMDir)
	default:
		return config.ConnectionInfo{}, fmt.Errorf("unknown backend type %q", req.BackendType)
	}

	if err != nil {
		return config.ConnectionInfo{}, err
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	a.Manager.ReplaceBackend(ctx, b)
	return a.Manager.ConnectionInfo(), nil
}

// Disconnect tears down the active backend.
func (a *App) Disconnect() error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.Manager.Disconnect(ctx)
}

// ConnectionInfo returns the current connection state and capabilities.
// A zero-value (DisplayName == "") means not connected.
func (a *App) ConnectionInfo() config.ConnectionInfo {
	return a.Manager.ConnectionInfo()
}

// --- Settings ---

func (a *App) LoadConnectionSettings() config.ConnectRequest {
	req, _ := config.LoadConnection()
	return req
}

func (a *App) SaveConnectionSettings(req config.ConnectRequest) error {
	return config.SaveConnection(req)
}

func (a *App) ClearConnectionSettings() error {
	return config.ClearConnection()
}

// --- File dialogs ---

func (a *App) OpenFileDialog(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

func (a *App) OpenDirectoryDialog(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

func (a *App) SaveFileDialog(title, defaultFilename string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultFilename,
	})
}

func (a *App) OpenVMConsole(vmRef string) error {
	consoleURL, err := a.Manager.VMConsoleURL(vmRef)
	if err != nil {
		return err
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	runtime.BrowserOpenURL(ctx, consoleURL)
	return nil
}

// --- SSH Key Management ---

func (a *App) CreateSSHKey(label, algorithm, defaultUser string) (config.KeyMeta, error) {
	return config.CreateKeyPair(label, algorithm, defaultUser)
}

func (a *App) ListSSHKeys() ([]config.KeyMeta, error) {
	return config.ListKeys()
}

func (a *App) DeleteSSHKey(label string) error {
	return config.DeleteKey(label)
}

// --- Guest Credential Management ---

func (a *App) CreateGuestCredential(label, username, password string) (config.GuestCredentialMeta, error) {
	return config.CreateGuestCredential(label, username, password)
}

func (a *App) ListGuestCredentials() ([]config.GuestCredentialMeta, error) {
	return config.ListGuestCredentials()
}

func (a *App) GetGuestCredential(label string) (config.GuestCredential, error) {
	return config.LoadGuestCredential(label)
}

func (a *App) UpdateGuestCredential(currentLabel, newLabel, username, password string) (config.GuestCredentialMeta, error) {
	return config.UpdateGuestCredential(currentLabel, newLabel, username, password)
}

func (a *App) DeleteGuestCredential(label string) error {
	return config.DeleteGuestCredential(label)
}

// --- Stored Script Management ---

func (a *App) ListScripts() (config.ScriptCatalog, error) {
	return config.ListScripts()
}

func (a *App) GetScript(id string) (config.StoredScript, error) {
	return config.LoadScript(id)
}

func (a *App) SaveScript(req config.ScriptSaveRequest) (config.StoredScript, error) {
	return config.SaveScript(req)
}

func (a *App) DeleteScript(id string) error {
	return config.DeleteScript(id)
}

package manager

// API is the Wails-facing surface for VM feature operations.
// It intentionally exposes only methods that the frontend calls directly.
type API struct {
	manager *Manager
}

func NewAPI(manager *Manager) *API {
	return &API{manager: manager}
}

func (api *API) DeploySSHKey(req DeploySSHKeyRequest) string {
	return api.manager.deploySSHKey(req)
}

func (api *API) Download(req DownloadRequest) string {
	return api.manager.download(req)
}

func (api *API) GuestRun(req RunRequest) string {
	return api.manager.guestRun(req)
}

func (api *API) InventoryDatastores() ([]DatastoreInfo, error) {
	return api.manager.inventoryDatastores()
}

func (api *API) InventoryHosts() ([]HostInfo, error) {
	return api.manager.inventoryHosts()
}

func (api *API) InventoryNetworks() (NetworkSummary, error) {
	return api.manager.inventoryNetworks()
}

func (api *API) SnapshotCreate(req CreateSnapshotRequest) string {
	return api.manager.snapshotCreate(req)
}

func (api *API) SnapshotDelete(snapRef string, removeChildren bool) string {
	return api.manager.snapshotDelete(snapRef, removeChildren)
}

func (api *API) SnapshotList(vmRef string) ([]SnapshotInfo, error) {
	return api.manager.snapshotList(vmRef)
}

func (api *API) SnapshotRevert(snapRef string) string {
	return api.manager.snapshotRevert(snapRef)
}

func (api *API) SSHDownload(req SSHTransferRequest) string {
	return api.manager.sshDownload(req)
}

func (api *API) SSHRun(req SSHRunRequest) string {
	return api.manager.sshRun(req)
}

func (api *API) SSHUpload(req SSHTransferRequest) string {
	return api.manager.sshUpload(req)
}

func (api *API) Upload(req UploadRequest) string {
	return api.manager.upload(req)
}

func (api *API) VMConsoleInfo(vmRef string) (ConsoleLaunchInfo, error) {
	return api.manager.vmConsoleInfo(vmRef)
}

func (api *API) VMGet(vmRef string) (VMInfo, error) {
	return api.manager.vmGet(vmRef)
}

func (api *API) VMInstallTools(vmRef string) string {
	return api.manager.vmInstallTools(vmRef)
}

func (api *API) VMList() ([]VMInfo, error) {
	return api.manager.vmList()
}

func (api *API) VMNetworkOptions(vmRef string) ([]VMNetworkOption, error) {
	return api.manager.vmNetworkOptions(vmRef)
}

func (api *API) VMPowerOff(vmRef string) string {
	return api.manager.vmPowerOff(vmRef)
}

func (api *API) VMPowerOn(vmRef string) string {
	return api.manager.vmPowerOn(vmRef)
}

func (api *API) VMReset(vmRef string) string {
	return api.manager.vmReset(vmRef)
}

func (api *API) VMSuspend(vmRef string) string {
	return api.manager.vmSuspend(vmRef)
}

func (api *API) VMUpdateConfig(req VMConfigUpdateRequest) string {
	return api.manager.vmUpdateConfig(req)
}

func (api *API) VMUpdateNetwork(req VMNetworkUpdateRequest) string {
	return api.manager.vmUpdateNetwork(req)
}

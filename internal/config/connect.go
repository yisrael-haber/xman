package config

// ConnectRequest is the full payload for establishing a connection.
// BackendType determines which fields are relevant.
type ConnectRequest struct {
	BackendType string `json:"backendType"` // "vcenter" | "workstation"

	// vCenter fields
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Insecure bool   `json:"insecure"`

	// Workstation fields (both auto-detected/defaulted if empty)
	VmrunPath string `json:"vmrunPath"`
	VMDir     string `json:"vmDir"` // custom VM search directory; falls back to defaults if empty
}

// ConnectionInfo is returned after a successful connect.
// A zero-value (DisplayName == "") signals not connected.
type ConnectionInfo struct {
	BackendType  string `json:"backendType"`
	DisplayName  string `json:"displayName"`
	GuestOps     bool   `json:"guestOps"`     // file transfer + guest exec available
	Inventory    bool   `json:"inventory"`    // host + datastore inventory available
	ToolsInstall bool   `json:"toolsInstall"` // VMware Tools install/upgrade available
	Console      bool   `json:"console"`      // browser console launch available
}

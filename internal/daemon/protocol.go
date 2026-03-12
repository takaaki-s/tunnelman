// Package daemon provides the tunnelman daemon server and client.
package daemon

import "encoding/json"

// Action constants for daemon protocol.
const (
	ActionAdd           = "add"
	ActionRemove        = "rm"
	ActionEdit          = "edit"
	ActionStart         = "start"
	ActionStop          = "stop"
	ActionStartAll      = "start_all"
	ActionStopAll       = "stop_all"
	ActionList          = "list"
	ActionStatus        = "status"
	ActionDaemonStatus  = "daemon_status"
	ActionShutdown      = "shutdown"
	ActionProfileList   = "profile_list"
	ActionProfileCreate = "profile_create"
	ActionProfileRemove = "profile_rm"
	ActionImport        = "import"
)

// Request is the JSON message sent from client to daemon.
type Request struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// Response is the JSON message sent from daemon to client.
type Response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// AddRequest adds a tunnel to the config.
type AddRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	SSHHost     string   `json:"ssh_host"`
	LocalHost   string   `json:"local_host"`
	LocalPort   int      `json:"local_port"`
	RemoteHost  string   `json:"remote_host"`
	RemotePort  int      `json:"remote_port"`
	Profile     string   `json:"profile"`
	AutoConnect bool     `json:"auto_connect"`
	SSHOptions  []string `json:"ssh_options,omitempty"`
}

// RemoveRequest removes a tunnel by ID.
type RemoveRequest struct {
	ID string `json:"id"`
}

// EditRequest edits an existing tunnel.
type EditRequest struct {
	ID          string   `json:"id"`
	Name        *string  `json:"name,omitempty"`
	Type        *string  `json:"type,omitempty"`
	SSHHost     *string  `json:"ssh_host,omitempty"`
	LocalHost   *string  `json:"local_host,omitempty"`
	LocalPort   *int     `json:"local_port,omitempty"`
	RemoteHost  *string  `json:"remote_host,omitempty"`
	RemotePort  *int     `json:"remote_port,omitempty"`
	Profile     *string  `json:"profile,omitempty"`
	AutoConnect *bool    `json:"auto_connect,omitempty"`
	SSHOptions  []string `json:"ssh_options,omitempty"`
}

// StartRequest starts a tunnel by ID.
type StartRequest struct {
	ID string `json:"id"`
}

// StopRequest stops a tunnel by ID.
type StopRequest struct {
	ID string `json:"id"`
}

// ListRequest lists tunnels with optional filters.
type ListRequest struct {
	Profile string `json:"profile,omitempty"`
	Status  string `json:"status,omitempty"`
}

// StatusRequest gets status for a specific tunnel.
type StatusRequest struct {
	ID string `json:"id"`
}

// TunnelInfo is the combined config + runtime info returned for list/status.
type TunnelInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	SSHHost        string `json:"ssh_host"`
	LocalHost      string `json:"local_host"`
	LocalPort      int    `json:"local_port"`
	RemoteHost     string `json:"remote_host"`
	RemotePort     int    `json:"remote_port"`
	Profile        string `json:"profile"`
	AutoConnect    bool   `json:"auto_connect"`
	Status         string `json:"status"`
	PID            int    `json:"pid,omitempty"`
	ReconnectCount int    `json:"reconnect_count,omitempty"`
}

// ListResponse is the response for the list action.
type ListResponse struct {
	Tunnels []TunnelInfo `json:"tunnels"`
}

// DaemonStatusResponse returns daemon-level information.
type DaemonStatusResponse struct {
	PID       int    `json:"pid"`
	Uptime    string `json:"uptime"`
	Tunnels   int    `json:"tunnels"`
	Running   int    `json:"running"`
	ConfigPath string `json:"config_path"`
}

// ProfileCreateRequest creates a new profile.
type ProfileCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ProfileRemoveRequest removes a profile.
type ProfileRemoveRequest struct {
	Name string `json:"name"`
}

// ProfileInfo is returned for profile list.
type ProfileInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TunnelCount int    `json:"tunnel_count"`
}

// ProfileListResponse is the response for profile_list.
type ProfileListResponse struct {
	Profiles []ProfileInfo `json:"profiles"`
}

// ImportRequest imports tunnels from SSH config.
type ImportRequest struct {
	SSHConfigPath string `json:"ssh_config_path"`
	HostAlias     string `json:"host_alias"`
	Profile       string `json:"profile"`
}

// ImportResponse returns import results.
type ImportResponse struct {
	Imported int `json:"imported"`
}

package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
)

// Client communicates with the daemon via Unix socket.
type Client struct {
	socketPath string
}

// NewClient creates a new daemon client.
func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// IsRunning checks if the daemon is reachable.
func (c *Client) IsRunning() bool {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (c *Client) send(req Request) (*Response, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("daemon not running. Start with: tunnelman daemon start")
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) sendAction(action string, data any) error {
	var rawData json.RawMessage
	if data != nil {
		rawData, _ = json.Marshal(data)
	}
	resp, err := c.send(Request{Action: action, Data: rawData})
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Error)
	}
	return nil
}

func (c *Client) sendActionWithResponse(action string, data any) (*Response, error) {
	var rawData json.RawMessage
	if data != nil {
		rawData, _ = json.Marshal(data)
	}
	resp, err := c.send(Request{Action: action, Data: rawData})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errors.New(resp.Error)
	}
	return resp, nil
}

// Add adds a tunnel to the daemon config.
func (c *Client) Add(req AddRequest) error {
	return c.sendAction(ActionAdd, req)
}

// Remove removes a tunnel by ID.
func (c *Client) Remove(id string) error {
	return c.sendAction(ActionRemove, RemoveRequest{ID: id})
}

// Edit edits a tunnel.
func (c *Client) Edit(req EditRequest) error {
	return c.sendAction(ActionEdit, req)
}

// Start starts a tunnel by ID.
func (c *Client) Start(id string) error {
	return c.sendAction(ActionStart, StartRequest{ID: id})
}

// Stop stops a tunnel by ID.
func (c *Client) Stop(id string) error {
	return c.sendAction(ActionStop, StopRequest{ID: id})
}

// StartAll starts all tunnels.
func (c *Client) StartAll() error {
	return c.sendAction(ActionStartAll, nil)
}

// StopAll stops all tunnels.
func (c *Client) StopAll() error {
	return c.sendAction(ActionStopAll, nil)
}

// List returns tunnels matching optional filters.
func (c *Client) List(req ListRequest) (*ListResponse, error) {
	resp, err := c.sendActionWithResponse(ActionList, req)
	if err != nil {
		return nil, err
	}
	var lr ListResponse
	if err := json.Unmarshal(resp.Data, &lr); err != nil {
		return nil, err
	}
	return &lr, nil
}

// Status returns status for a specific tunnel.
func (c *Client) Status(id string) (*TunnelInfo, error) {
	resp, err := c.sendActionWithResponse(ActionStatus, StatusRequest{ID: id})
	if err != nil {
		return nil, err
	}
	var info TunnelInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// DaemonStatus returns daemon-level information.
func (c *Client) DaemonStatus() (*DaemonStatusResponse, error) {
	resp, err := c.sendActionWithResponse(ActionDaemonStatus, nil)
	if err != nil {
		return nil, err
	}
	var ds DaemonStatusResponse
	if err := json.Unmarshal(resp.Data, &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

// Shutdown requests the daemon to shut down.
func (c *Client) Shutdown() error {
	return c.sendAction(ActionShutdown, nil)
}

// ProfileList returns all profiles.
func (c *Client) ProfileList() (*ProfileListResponse, error) {
	resp, err := c.sendActionWithResponse(ActionProfileList, nil)
	if err != nil {
		return nil, err
	}
	var plr ProfileListResponse
	if err := json.Unmarshal(resp.Data, &plr); err != nil {
		return nil, err
	}
	return &plr, nil
}

// ProfileCreate creates a new profile.
func (c *Client) ProfileCreate(name, description string) error {
	return c.sendAction(ActionProfileCreate, ProfileCreateRequest{Name: name, Description: description})
}

// ProfileRemove removes a profile.
func (c *Client) ProfileRemove(name string) error {
	return c.sendAction(ActionProfileRemove, ProfileRemoveRequest{Name: name})
}

// Import imports tunnels from SSH config.
func (c *Client) Import(req ImportRequest) (*ImportResponse, error) {
	resp, err := c.sendActionWithResponse(ActionImport, req)
	if err != nil {
		return nil, err
	}
	var ir ImportResponse
	if err := json.Unmarshal(resp.Data, &ir); err != nil {
		return nil, err
	}
	return &ir, nil
}

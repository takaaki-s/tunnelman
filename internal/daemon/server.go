package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/takaaki-s/tunnelman/internal/config"
	"github.com/takaaki-s/tunnelman/internal/tunnel"
)

// Server is the tunnelman daemon that manages SSH tunnel processes.
type Server struct {
	socketPath     string
	config         *config.Config
	configPath     string
	state          *config.StateManager
	processManager *ProcessManager
	listener       net.Listener
	done           chan struct{}
	startedAt      time.Time
	mu             sync.Mutex
}

// NewServer creates a new daemon server.
func NewServer(socketPath string, cfg *config.Config, configPath string, state *config.StateManager) *Server {
	return &Server{
		socketPath:     socketPath,
		config:         cfg,
		configPath:     configPath,
		state:          state,
		processManager: NewProcessManager(nil),
		done:           make(chan struct{}),
		startedAt:      time.Now(),
	}
}

// SetProcessManager replaces the process manager (for testing).
func (s *Server) SetProcessManager(pm *ProcessManager) {
	s.processManager = pm
}

// Start begins listening on the Unix socket.
func (s *Server) Start() error {
	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.socketPath, err)
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}
		go s.handleConnection(conn)
	}
}

// Stop shuts down the server.
func (s *Server) Stop() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}

	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		ln.Close()
	}
	s.processManager.StopAll()
	os.Remove(s.socketPath)
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	var resp Response

	switch req.Action {
	case ActionAdd:
		resp = s.handleAdd(req.Data)
	case ActionRemove:
		resp = s.handleRemove(req.Data)
	case ActionEdit:
		resp = s.handleEdit(req.Data)
	case ActionStart:
		resp = s.handleStart(req.Data)
	case ActionStop:
		resp = s.handleStop(req.Data)
	case ActionStartAll:
		resp = s.handleStartAll()
	case ActionStopAll:
		resp = s.handleStopAll()
	case ActionList:
		resp = s.handleList(req.Data)
	case ActionStatus:
		resp = s.handleStatus(req.Data)
	case ActionDaemonStatus:
		resp = s.handleDaemonStatus()
	case ActionShutdown:
		resp = Response{Success: true}
		_ = json.NewEncoder(conn).Encode(resp)
		go s.Stop()
		return
	case ActionProfileList:
		resp = s.handleProfileList()
	case ActionProfileCreate:
		resp = s.handleProfileCreate(req.Data)
	case ActionProfileRemove:
		resp = s.handleProfileRemove(req.Data)
	case ActionImport:
		resp = s.handleImport(req.Data)
	default:
		resp = Response{Success: false, Error: fmt.Sprintf("unknown action: %s", req.Action)}
	}

	_ = json.NewEncoder(conn).Encode(resp)
}

func (s *Server) handleAdd(data json.RawMessage) Response {
	var ar AddRequest
	if err := json.Unmarshal(data, &ar); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check duplicate ID
	for _, t := range s.config.Tunnels {
		if t.ID == ar.ID {
			return Response{Success: false, Error: fmt.Sprintf("tunnel ID %s already exists", ar.ID)}
		}
	}

	tc := config.TunnelConfig{
		ID: ar.ID, Name: ar.Name, Type: ar.Type,
		SSHHost: ar.SSHHost, LocalHost: ar.LocalHost,
		LocalPort: ar.LocalPort, RemoteHost: ar.RemoteHost,
		RemotePort: ar.RemotePort, Profile: ar.Profile,
		AutoConnect: ar.AutoConnect, SSHOptions: ar.SSHOptions,
	}

	// Validate via tunnel package
	tun := configToTunnel(tc)
	if err := tun.Validate(); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	s.config.Tunnels = append(s.config.Tunnels, tc)
	if err := s.saveConfig(); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	return Response{Success: true}
}

func (s *Server) handleRemove(data json.RawMessage) Response {
	var rr RemoveRequest
	if err := json.Unmarshal(data, &rr); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.findTunnelIndex(rr.ID)
	if idx < 0 {
		return Response{Success: false, Error: fmt.Sprintf("tunnel %s not found", rr.ID)}
	}

	// Disconnect can be called under s.mu because onExit is asynchronous (go onExit()).
	if s.processManager.IsRunning(rr.ID) {
		_ = s.processManager.Disconnect(rr.ID)
		s.state.RemoveTunnel(rr.ID)
		_ = s.state.Save()
	}

	s.config.Tunnels = append(s.config.Tunnels[:idx], s.config.Tunnels[idx+1:]...)
	if err := s.saveConfig(); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	return Response{Success: true}
}

func (s *Server) handleEdit(data json.RawMessage) Response {
	var er EditRequest
	if err := json.Unmarshal(data, &er); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.findTunnelIndex(er.ID)
	if idx < 0 {
		return Response{Success: false, Error: fmt.Sprintf("tunnel %s not found", er.ID)}
	}

	tc := &s.config.Tunnels[idx]
	if er.Name != nil {
		tc.Name = *er.Name
	}
	if er.Type != nil {
		tc.Type = *er.Type
	}
	if er.SSHHost != nil {
		tc.SSHHost = *er.SSHHost
	}
	if er.LocalHost != nil {
		tc.LocalHost = *er.LocalHost
	}
	if er.LocalPort != nil {
		tc.LocalPort = *er.LocalPort
	}
	if er.RemoteHost != nil {
		tc.RemoteHost = *er.RemoteHost
	}
	if er.RemotePort != nil {
		tc.RemotePort = *er.RemotePort
	}
	if er.Profile != nil {
		tc.Profile = *er.Profile
	}
	if er.AutoConnect != nil {
		tc.AutoConnect = *er.AutoConnect
	}
	if er.SSHOptions != nil {
		tc.SSHOptions = er.SSHOptions
	}

	// Validate
	tun := configToTunnel(*tc)
	if err := tun.Validate(); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	if err := s.saveConfig(); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	return Response{Success: true}
}

func (s *Server) handleStart(data json.RawMessage) Response {
	var sr StartRequest
	if err := json.Unmarshal(data, &sr); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	s.mu.Lock()
	idx := s.findTunnelIndex(sr.ID)
	if idx < 0 {
		s.mu.Unlock()
		return Response{Success: false, Error: fmt.Sprintf("tunnel %s not found", sr.ID)}
	}
	tun := configToTunnel(s.config.Tunnels[idx])
	s.mu.Unlock()

	// Connect outside s.mu to avoid lock-ordering issues with ProcessManager/onExit.
	info, err := s.processManager.Connect(tun)
	if err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	s.state.SetTunnel(sr.ID, &config.TunnelState{
		PID:       info.PID,
		StartedAt: info.StartedAt,
		Status:    "running",
	})
	_ = s.state.Save()

	return Response{Success: true}
}

func (s *Server) handleStop(data json.RawMessage) Response {
	var sr StopRequest
	if err := json.Unmarshal(data, &sr); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	if !s.processManager.IsRunning(sr.ID) {
		return Response{Success: false, Error: fmt.Sprintf("tunnel %s is not running", sr.ID)}
	}

	// Disconnect outside s.mu to avoid lock-ordering issues with ProcessManager/onExit.
	if err := s.processManager.Disconnect(sr.ID); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	s.state.RemoveTunnel(sr.ID)
	_ = s.state.Save()

	return Response{Success: true}
}

func (s *Server) handleStartAll() Response {
	// Collect tunnels to start under lock, then connect outside lock.
	s.mu.Lock()
	var toStart []*tunnel.Tunnel
	for _, tc := range s.config.Tunnels {
		if s.processManager.IsRunning(tc.ID) {
			continue
		}
		toStart = append(toStart, configToTunnel(tc))
	}
	s.mu.Unlock()

	started := 0
	for _, tun := range toStart {
		info, err := s.processManager.Connect(tun)
		if err != nil {
			slog.Warn("failed to start tunnel", "id", tun.ID, "error", err)
			continue
		}
		s.state.SetTunnel(tun.ID, &config.TunnelState{
			PID:       info.PID,
			StartedAt: info.StartedAt,
			Status:    "running",
		})
		started++
	}
	_ = s.state.Save()

	respData, _ := json.Marshal(map[string]int{"started": started})
	return Response{Success: true, Data: respData}
}

func (s *Server) handleStopAll() Response {
	// Collect IDs to stop under lock, then disconnect outside lock.
	s.mu.Lock()
	var toStop []string
	for _, tc := range s.config.Tunnels {
		if s.processManager.IsRunning(tc.ID) {
			toStop = append(toStop, tc.ID)
		}
	}
	s.mu.Unlock()

	stopped := 0
	for _, id := range toStop {
		if err := s.processManager.Disconnect(id); err == nil {
			s.state.RemoveTunnel(id)
			stopped++
		}
	}
	_ = s.state.Save()

	respData, _ := json.Marshal(map[string]int{"stopped": stopped})
	return Response{Success: true, Data: respData}
}

func (s *Server) handleList(data json.RawMessage) Response {
	var lr ListRequest
	if data != nil {
		_ = json.Unmarshal(data, &lr)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var tunnels []TunnelInfo
	for _, tc := range s.config.Tunnels {
		if lr.Profile != "" && tc.Profile != lr.Profile {
			continue
		}

		status := "stopped"
		var pid int
		var reconnectCount int
		if ts := s.state.GetTunnel(tc.ID); ts != nil {
			status = ts.Status
			pid = ts.PID
			reconnectCount = ts.ReconnectCount
		}
		if s.processManager.IsRunning(tc.ID) && status == "stopped" {
			status = "running"
		}

		if lr.Status != "" && status != lr.Status {
			continue
		}

		tunnels = append(tunnels, TunnelInfo{
			ID: tc.ID, Name: tc.Name, Type: tc.Type,
			SSHHost: tc.SSHHost, LocalHost: tc.LocalHost,
			LocalPort: tc.LocalPort, RemoteHost: tc.RemoteHost,
			RemotePort: tc.RemotePort, Profile: tc.Profile,
			AutoConnect: tc.AutoConnect,
			Status: status, PID: pid,
			ReconnectCount: reconnectCount,
		})
	}

	if tunnels == nil {
		tunnels = []TunnelInfo{}
	}

	respData, _ := json.Marshal(ListResponse{Tunnels: tunnels})
	return Response{Success: true, Data: respData}
}

func (s *Server) handleStatus(data json.RawMessage) Response {
	var sr StatusRequest
	if err := json.Unmarshal(data, &sr); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.findTunnelIndex(sr.ID)
	if idx < 0 {
		return Response{Success: false, Error: fmt.Sprintf("tunnel %s not found", sr.ID)}
	}

	tc := s.config.Tunnels[idx]
	status := "stopped"
	var pid int
	if ts := s.state.GetTunnel(tc.ID); ts != nil {
		status = ts.Status
		pid = ts.PID
	}

	info := TunnelInfo{
		ID: tc.ID, Name: tc.Name, Type: tc.Type,
		SSHHost: tc.SSHHost, LocalHost: tc.LocalHost,
		LocalPort: tc.LocalPort, RemoteHost: tc.RemoteHost,
		RemotePort: tc.RemotePort, Profile: tc.Profile,
		AutoConnect: tc.AutoConnect,
		Status: status, PID: pid,
	}

	respData, _ := json.Marshal(info)
	return Response{Success: true, Data: respData}
}

func (s *Server) handleDaemonStatus() Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	running := 0
	for _, tc := range s.config.Tunnels {
		if s.processManager.IsRunning(tc.ID) {
			running++
		}
	}

	ds := DaemonStatusResponse{
		PID:        os.Getpid(),
		Uptime:     time.Since(s.startedAt).Round(time.Second).String(),
		Tunnels:    len(s.config.Tunnels),
		Running:    running,
		ConfigPath: s.configPath,
	}
	respData, _ := json.Marshal(ds)
	return Response{Success: true, Data: respData}
}

func (s *Server) handleProfileList() Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	var profiles []ProfileInfo
	for _, p := range s.config.Profiles {
		count := 0
		for _, t := range s.config.Tunnels {
			if t.Profile == p.Name {
				count++
			}
		}
		profiles = append(profiles, ProfileInfo{
			Name: p.Name, Description: p.Description, TunnelCount: count,
		})
	}

	if profiles == nil {
		profiles = []ProfileInfo{}
	}

	respData, _ := json.Marshal(ProfileListResponse{Profiles: profiles})
	return Response{Success: true, Data: respData}
}

func (s *Server) handleProfileCreate(data json.RawMessage) Response {
	var pr ProfileCreateRequest
	if err := json.Unmarshal(data, &pr); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.config.Profiles {
		if p.Name == pr.Name {
			return Response{Success: false, Error: fmt.Sprintf("profile %s already exists", pr.Name)}
		}
	}

	s.config.Profiles = append(s.config.Profiles, config.ProfileConfig{
		Name: pr.Name, Description: pr.Description,
	})
	if err := s.saveConfig(); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	return Response{Success: true}
}

func (s *Server) handleProfileRemove(data json.RawMessage) Response {
	var pr ProfileRemoveRequest
	if err := json.Unmarshal(data, &pr); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, p := range s.config.Profiles {
		if p.Name == pr.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Response{Success: false, Error: fmt.Sprintf("profile %s not found", pr.Name)}
	}

	// Check if tunnels use this profile
	for _, t := range s.config.Tunnels {
		if t.Profile == pr.Name {
			return Response{Success: false, Error: fmt.Sprintf("profile %s is in use by tunnel %s", pr.Name, t.ID)}
		}
	}

	s.config.Profiles = append(s.config.Profiles[:idx], s.config.Profiles[idx+1:]...)
	if err := s.saveConfig(); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	return Response{Success: true}
}

func (s *Server) handleImport(data json.RawMessage) Response {
	var ir ImportRequest
	if err := json.Unmarshal(data, &ir); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("invalid request: %v", err)}
	}

	// Import is handled at cmd layer where sshconfig package is used.
	// The daemon just receives pre-converted tunnel configs via AddRequest.
	// This handler is a placeholder for direct import support.
	return Response{Success: false, Error: "import should be performed via CLI using add commands"}
}

// Helper methods

func (s *Server) findTunnelIndex(id string) int {
	for i, t := range s.config.Tunnels {
		if t.ID == id {
			return i
		}
	}
	return -1
}

func (s *Server) saveConfig() error {
	if s.configPath == "" {
		return nil
	}
	return config.SaveConfig(s.configPath, s.config)
}

func configToTunnel(tc config.TunnelConfig) *tunnel.Tunnel {
	return &tunnel.Tunnel{
		ID: tc.ID, Name: tc.Name,
		Type:        tunnel.TunnelType(tc.Type),
		SSHHost:     tc.SSHHost,
		LocalHost:   tc.LocalHost,
		LocalPort:   tc.LocalPort,
		RemoteHost:  tc.RemoteHost,
		RemotePort:  tc.RemotePort,
		Profile:     tc.Profile,
		AutoConnect: tc.AutoConnect,
		SSHOptions:  tc.SSHOptions,
	}
}

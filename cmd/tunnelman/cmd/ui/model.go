package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

type viewState int

const (
	viewList viewState = iota
	viewForm
	viewDelete
	viewHelp
)

// Model is the main TUI model.
type Model struct {
	client          *daemon.Client
	tunnels         []daemon.TunnelInfo
	cursor          int
	scrollOffset    int
	state           viewState
	form            FormModel
	profiles        []string
	selectedProfile int
	statusMsg       string
	statusIsErr     bool
	width           int
	height          int
}

// --- Msg types ---

type tickMsg time.Time
type tunnelsLoadedMsg []daemon.TunnelInfo
type profilesLoadedMsg []string
type actionDoneMsg string
type partialErrMsg string
type errMsg struct{ err error }

// NewModel creates a new TUI model.
func NewModel(client *daemon.Client) Model {
	return Model{
		client:   client,
		profiles: []string{"All"},
		width:    80,
		height:   24,
	}
}

// Init starts the initial data fetches and the polling ticker.
// tickEvery starts a self-renewing 1-second poll: each tickMsg handler re-issues
// the next tick, so Init must be called exactly once by bubbletea.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchTunnels(m.client, ""),
		fetchProfiles(m.client),
		tickEvery(time.Second),
	)
}

// Update handles messages and key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(
			fetchTunnels(m.client, m.selectedProfileName()),
			tickEvery(time.Second),
		)

	case tunnelsLoadedMsg:
		m.tunnels = msg
		if len(m.tunnels) == 0 {
			m.cursor = 0
			m.scrollOffset = 0
		} else {
			if m.cursor >= len(m.tunnels) {
				m.cursor = len(m.tunnels) - 1
			}
			if m.scrollOffset >= len(m.tunnels) {
				m.scrollOffset = max(0, len(m.tunnels)-1)
			}
		}
		return m, nil

	case profilesLoadedMsg:
		names := []string{"All"}
		names = append(names, msg...)
		m.profiles = names
		return m, nil

	case actionDoneMsg:
		m.statusMsg = string(msg)
		m.statusIsErr = false
		// Immediate refresh after action; may overlap with polling tick — intentional.
		return m, fetchTunnels(m.client, m.selectedProfileName())

	case partialErrMsg:
		m.statusMsg = string(msg)
		m.statusIsErr = true
		// Immediate refresh after partial failure — intentional overlap with polling.
		return m, fetchTunnels(m.client, m.selectedProfileName())

	case errMsg:
		// Don't overwrite an existing action success message with a polling error
		if m.statusMsg == "" || m.statusIsErr {
			m.statusMsg = msg.err.Error()
			m.statusIsErr = true
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case viewHelp:
		m.state = viewList
		return m, nil

	case viewDelete:
		return m.handleDeleteKey(msg)

	case viewForm:
		return m.handleFormKey(msg)

	default:
		return m.handleListKey(msg)
	}
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyQuit, keyCtrlC:
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
		}

	case "down", "j":
		if m.cursor < len(m.tunnels)-1 {
			m.cursor++
			tableHeight := m.tableHeight()
			if m.cursor >= m.scrollOffset+tableHeight {
				m.scrollOffset = m.cursor - tableHeight + 1
			}
		}

	case "tab":
		m.selectedProfile = (m.selectedProfile + 1) % len(m.profiles)
		m.cursor = 0
		m.scrollOffset = 0
		return m, fetchTunnels(m.client, m.selectedProfileName())

	case "shift+tab":
		m.selectedProfile = (m.selectedProfile - 1 + len(m.profiles)) % len(m.profiles)
		m.cursor = 0
		m.scrollOffset = 0
		return m, fetchTunnels(m.client, m.selectedProfileName())

	case keyStart:
		if t := m.selectedTunnel(); t != nil {
			return m, startTunnel(m.client, t.ID)
		}

	case keyStop:
		if t := m.selectedTunnel(); t != nil {
			return m, stopTunnel(m.client, t.ID)
		}

	case keyStartAll:
		if m.selectedProfile == 0 {
			return m, startAllTunnels(m.client)
		}
		return m, startAllInProfile(m.client, m.tunnels)

	case keyStopAll:
		if m.selectedProfile == 0 {
			return m, stopAllTunnels(m.client)
		}
		return m, stopAllInProfile(m.client, m.tunnels)

	case keyAdd:
		m.form = NewForm()
		m.state = viewForm

	case keyEdit:
		if t := m.selectedTunnel(); t != nil {
			m.form = NewEditForm(*t)
			m.state = viewForm
		}

	case keyDelete:
		if m.selectedTunnel() != nil {
			m.state = viewDelete
		}

	case keyRefresh:
		m.statusMsg = ""
		return m, fetchTunnels(m.client, m.selectedProfileName())

	case keyHelp:
		m.state = viewHelp
	}

	return m, nil
}

func (m Model) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		t := m.selectedTunnel()
		if t == nil {
			m.state = viewList
			return m, nil
		}
		id := t.ID
		m.state = viewList
		return m, removeTunnel(m.client, id)
	case keyCtrlC:
		return m, tea.Quit
	case keyQuit, "n", "N":
		// "q" cancels the delete (not quit) to prevent accidental quit during confirmation
		m.state = viewList
		return m, nil
	default:
		m.state = viewList
		return m, nil
	}
}

func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyCtrlC:
		return m, tea.Quit

	case "esc":
		m.state = viewList
		return m, nil

	case "shift+tab":
		var cmd tea.Cmd
		m.form, cmd = m.form.FocusPrev()
		return m, cmd

	case "tab":
		var cmd tea.Cmd
		m.form, cmd = m.form.FocusNext()
		return m, cmd

	case "enter":
		if m.form.IsLastField() {
			return m.submitForm()
		}
		var cmd tea.Cmd
		m.form, cmd = m.form.FocusNext()
		return m, cmd

	default:
		// Pass keystrokes to the focused textinput
		var cmd tea.Cmd
		m.form, cmd = m.form.UpdateFocused(msg)
		return m, cmd
	}
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	m.form = m.form.ClearError()
	if m.form.isEdit {
		req, err := m.form.ToEditRequest()
		if err != nil {
			m.form = m.form.WithError(err.Error())
			return m, nil
		}
		m.state = viewList
		return m, editTunnel(m.client, req)
	}
	req, err := m.form.ToAddRequest()
	if err != nil {
		m.form = m.form.WithError(err.Error())
		return m, nil
	}
	m.state = viewList
	return m, addTunnel(m.client, req)
}

// View renders the entire TUI.
func (m Model) View() string {
	if m.state == viewHelp {
		return RenderHelp(m.width, m.height)
	}

	var sb strings.Builder

	// Title
	sb.WriteString(styleTitle.Render("tunnelman") + "\n")

	// Profile tabs
	sb.WriteString(RenderProfileTabs(m.profiles, m.selectedProfile) + "\n")
	sb.WriteString(separatorStyle.Render(strings.Repeat("─", m.width)) + "\n")

	switch m.state {
	case viewForm:
		sb.WriteString(m.form.Render(m.width))

	case viewDelete:
		t := m.selectedTunnel()
		name := ""
		if t != nil {
			name = t.Name
		}
		msg := fmt.Sprintf("\n  Delete tunnel '%s'? (y/N) ", name)
		sb.WriteString(styleError.Render(msg))

	default:
		// Table header
		sb.WriteString(RenderTableHeader() + "\n")

		// Tunnel rows
		tableH := m.tableHeight()
		sb.WriteString(RenderTable(m.tunnels, m.cursor, m.scrollOffset, tableH))

		// Fill remaining lines
		rendered := min(max(len(m.tunnels)-m.scrollOffset, 0), tableH)
		for i := rendered; i < tableH; i++ {
			sb.WriteString("\n")
		}

		// Detail panel
		sb.WriteString("\n")
		sb.WriteString(RenderDetail(m.selectedTunnel(), m.width))
	}

	// Status bar
	sb.WriteString("\n")
	statusText := m.buildStatusBar()
	sb.WriteString(styleStatusBar.Width(m.width).Render(statusText))

	return sb.String()
}

func (m Model) buildStatusBar() string {
	if m.statusMsg != "" {
		if m.statusIsErr {
			return styleError.Render("Error: " + m.statusMsg)
		}
		return styleInfo.Render(m.statusMsg)
	}
	return hintText
}

func (m Model) selectedTunnel() *daemon.TunnelInfo {
	if len(m.tunnels) == 0 || m.cursor >= len(m.tunnels) {
		return nil
	}
	return &m.tunnels[m.cursor]
}

func (m Model) selectedProfileName() string {
	if m.selectedProfile == 0 || m.selectedProfile >= len(m.profiles) {
		return ""
	}
	return m.profiles[m.selectedProfile]
}

const (
	headerLines = 4 // title + tabs + separator + table header
	footerLines = 8 // detail border(2) + detail content(4) + newline + statusbar
)

func (m Model) tableHeight() int {
	return max(1, m.height-headerLines-footerLines)
}

// --- Commands ---

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchTunnels(c *daemon.Client, profile string) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.List(daemon.ListRequest{Profile: profile})
		if err != nil {
			return errMsg{err}
		}
		return tunnelsLoadedMsg(resp.Tunnels)
	}
}

func fetchProfiles(c *daemon.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.ProfileList()
		if err != nil {
			return errMsg{err}
		}
		names := make([]string, len(resp.Profiles))
		for i, p := range resp.Profiles {
			names[i] = p.Name
		}
		return profilesLoadedMsg(names)
	}
}

func startTunnel(c *daemon.Client, id string) tea.Cmd {
	return func() tea.Msg {
		if err := c.Start(id); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg(fmt.Sprintf("Started: %s", id))
	}
}

func stopTunnel(c *daemon.Client, id string) tea.Cmd {
	return func() tea.Msg {
		if err := c.Stop(id); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg(fmt.Sprintf("Stopped: %s", id))
	}
}

func startAllTunnels(c *daemon.Client) tea.Cmd {
	return func() tea.Msg {
		if err := c.StartAll(); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg("Started all tunnels")
	}
}

func stopAllTunnels(c *daemon.Client) tea.Cmd {
	return func() tea.Msg {
		if err := c.StopAll(); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg("Stopped all tunnels")
	}
}

func startAllInProfile(c *daemon.Client, tunnels []daemon.TunnelInfo) tea.Cmd {
	// tunnels is already filtered by profile at the call site.
	// Operates on the locally cached tunnel list; may be slightly stale.
	return func() tea.Msg {
		var errs []error
		count := 0
		for _, t := range tunnels {
			if t.Status == "stopped" || t.Status == "error" || t.Status == "unhealthy" {
				if err := c.Start(t.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", t.ID, err))
				} else {
					count++
				}
			}
		}
		if len(errs) > 0 {
			return partialErrMsg(fmt.Sprintf("Started %d tunnel(s), %d error(s): %s", count, len(errs), errors.Join(errs...)))
		}
		return actionDoneMsg(fmt.Sprintf("Started %d tunnel(s)", count))
	}
}

func stopAllInProfile(c *daemon.Client, tunnels []daemon.TunnelInfo) tea.Cmd {
	// tunnels is already filtered by profile at the call site.
	// Operates on the locally cached tunnel list; may be slightly stale.
	return func() tea.Msg {
		var errs []error
		count := 0
		for _, t := range tunnels {
			if t.Status == "running" || t.Status == "starting" || t.Status == "reconnecting" || t.Status == "unhealthy" || t.Status == "error" {
				if err := c.Stop(t.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", t.ID, err))
				} else {
					count++
				}
			}
		}
		if len(errs) > 0 {
			return partialErrMsg(fmt.Sprintf("Stopped %d tunnel(s), %d error(s): %s", count, len(errs), errors.Join(errs...)))
		}
		return actionDoneMsg(fmt.Sprintf("Stopped %d tunnel(s)", count))
	}
}

func addTunnel(c *daemon.Client, req daemon.AddRequest) tea.Cmd {
	return func() tea.Msg {
		// Generate ID client-side: daemon accepts caller-supplied IDs (same convention as CLI commands).
		// Sanitize name to avoid invalid characters in the ID.
		safeName := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
				return r
			}
			return '-'
		}, req.Name)
		if strings.Trim(safeName, "-") == "" {
			safeName = "tunnel"
		}
		req.ID = fmt.Sprintf("%s-%d", safeName, time.Now().UnixMilli())
		if req.LocalHost == "" {
			req.LocalHost = "127.0.0.1"
		}
		if err := c.Add(req); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg(fmt.Sprintf("Added: %s", req.Name))
	}
}

func editTunnel(c *daemon.Client, req daemon.EditRequest) tea.Cmd {
	return func() tea.Msg {
		if err := c.Edit(req); err != nil {
			return errMsg{err}
		}
		name := req.ID
		if req.Name != nil {
			name = *req.Name
		}
		return actionDoneMsg(fmt.Sprintf("Updated: %s", name))
	}
}

func removeTunnel(c *daemon.Client, id string) tea.Cmd {
	return func() tea.Msg {
		if err := c.Remove(id); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg(fmt.Sprintf("Deleted: %s", id))
	}
}

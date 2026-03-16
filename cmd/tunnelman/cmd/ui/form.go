package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/takaaki-s/tunnelman/internal/daemon"
	"github.com/takaaki-s/tunnelman/internal/tunnel"
)

const (
	fieldName        = iota
	fieldType
	fieldSSHHost
	fieldLocalPort
	fieldRemoteHost
	fieldRemotePort
	fieldProfile
	fieldAutoConnect
	numFields
)

var fieldLabels = [numFields]string{
	"Name",
	"Type (local/remote/dynamic)",
	"SSH Host",
	"Local Port",
	"Remote Host",
	"Remote Port",
	"Profile",
	"Auto Connect (true/false)",
}

// FormModel holds the state for the add/edit form.
type FormModel struct {
	fields    [numFields]textinput.Model
	focused   int
	isEdit    bool
	editID    string
	localHost string // hidden: preserves LocalHost across edits (not exposed in form)
	errMsg    string
}

// NewForm creates a blank add form.
func NewForm() FormModel {
	return newForm(false, "", daemon.TunnelInfo{})
}

// NewEditForm creates a pre-filled edit form from an existing tunnel.
func NewEditForm(t daemon.TunnelInfo) FormModel {
	return newForm(true, t.ID, t)
}

func newForm(isEdit bool, id string, t daemon.TunnelInfo) FormModel {
	var fields [numFields]textinput.Model
	for i := range numFields {
		f := textinput.New()
		f.Prompt = ""
		fields[i] = f
	}

	if isEdit {
		fields[fieldName].SetValue(t.Name)
		fields[fieldType].SetValue(t.Type)
		fields[fieldSSHHost].SetValue(t.SSHHost)
		fields[fieldLocalPort].SetValue(strconv.Itoa(t.LocalPort))
		fields[fieldRemoteHost].SetValue(t.RemoteHost)
		if t.RemotePort > 0 {
			fields[fieldRemotePort].SetValue(strconv.Itoa(t.RemotePort))
		}
		fields[fieldProfile].SetValue(t.Profile)
		fields[fieldAutoConnect].SetValue(strconv.FormatBool(t.AutoConnect))
	} else {
		fields[fieldType].SetValue("local")
		fields[fieldAutoConnect].SetValue("false")
	}

	fields[0].Focus()
	return FormModel{
		fields:    fields,
		focused:   0,
		isEdit:    isEdit,
		editID:    id,
		localHost: t.LocalHost,
	}
}

// FocusNext moves focus to the next field.
func (f FormModel) FocusNext() (FormModel, tea.Cmd) {
	f.fields[f.focused].Blur()
	f.focused = (f.focused + 1) % numFields
	cmd := f.fields[f.focused].Focus()
	return f, cmd
}

// FocusPrev moves focus to the previous field.
func (f FormModel) FocusPrev() (FormModel, tea.Cmd) {
	f.fields[f.focused].Blur()
	f.focused = (f.focused - 1 + numFields) % numFields
	cmd := f.fields[f.focused].Focus()
	return f, cmd
}

// UpdateFocused passes a message to the focused textinput and returns the updated form.
func (f FormModel) UpdateFocused(msg tea.Msg) (FormModel, tea.Cmd) {
	var cmd tea.Cmd
	f.fields[f.focused], cmd = f.fields[f.focused].Update(msg)
	return f, cmd
}

// IsLastField returns true if the cursor is on the last field.
func (f FormModel) IsLastField() bool {
	return f.focused == numFields-1
}

// ToAddRequest converts form values to AddRequest. Returns error if invalid.
func (f FormModel) ToAddRequest() (daemon.AddRequest, error) {
	if err := f.validate(); err != nil {
		return daemon.AddRequest{}, err
	}
	localPort, _ := strconv.Atoi(f.fields[fieldLocalPort].Value())   // validated above
	remotePort, _ := strconv.Atoi(f.fields[fieldRemotePort].Value()) // validated above
	autoConnect := f.fields[fieldAutoConnect].Value() == "true"
	remoteHost := f.fields[fieldRemoteHost].Value()
	if remoteHost == "" && f.fields[fieldType].Value() == "local" {
		remoteHost = "127.0.0.1"
	}

	return daemon.AddRequest{
		Name:        f.fields[fieldName].Value(),
		Type:        f.fields[fieldType].Value(),
		SSHHost:     f.fields[fieldSSHHost].Value(),
		LocalPort:   localPort,
		RemoteHost:  remoteHost,
		RemotePort:  remotePort,
		Profile:     f.fields[fieldProfile].Value(),
		AutoConnect: autoConnect,
	}, nil
}

// ToEditRequest converts form values to EditRequest. Returns error if invalid.
func (f FormModel) ToEditRequest() (daemon.EditRequest, error) {
	if err := f.validate(); err != nil {
		return daemon.EditRequest{}, err
	}
	name := f.fields[fieldName].Value()
	typ := f.fields[fieldType].Value()
	sshHost := f.fields[fieldSSHHost].Value()
	localPort, _ := strconv.Atoi(f.fields[fieldLocalPort].Value()) // validated above
	remoteHost := f.fields[fieldRemoteHost].Value()
	if remoteHost == "" && typ == "local" {
		remoteHost = "127.0.0.1"
	}
	remotePort, _ := strconv.Atoi(f.fields[fieldRemotePort].Value()) // validated above
	profile := f.fields[fieldProfile].Value()
	autoConnect := f.fields[fieldAutoConnect].Value() == "true"

	lh := f.localHost
	return daemon.EditRequest{
		ID:          f.editID,
		Name:        &name,
		Type:        &typ,
		SSHHost:     &sshHost,
		LocalPort:   &localPort,
		LocalHost:   &lh,
		RemoteHost:  &remoteHost,
		RemotePort:  &remotePort,
		Profile:     &profile,
		AutoConnect: &autoConnect,
	}, nil
}

func (f FormModel) validate() error {
	localPortStr := f.fields[fieldLocalPort].Value()
	if localPortStr == "" {
		return fmt.Errorf("local port is required")
	}
	localPort, err := strconv.Atoi(localPortStr)
	if err != nil {
		return fmt.Errorf("invalid local port")
	}

	remotePortStr := f.fields[fieldRemotePort].Value()
	var remotePort int
	if remotePortStr != "" {
		remotePort, err = strconv.Atoi(remotePortStr)
		if err != nil {
			return fmt.Errorf("invalid remote port")
		}
	}

	// autoConnect: any value other than "true" is treated as false (form label enforces true/false)
	autoConnect := f.fields[fieldAutoConnect].Value() == "true"

	lh := f.localHost
	if lh == "" {
		lh = "127.0.0.1"
	}
	remoteHost := f.fields[fieldRemoteHost].Value()
	if remoteHost == "" && f.fields[fieldType].Value() == "local" {
		remoteHost = "127.0.0.1"
	}
	t := &tunnel.Tunnel{
		Name:        f.fields[fieldName].Value(),
		Type:        tunnel.TunnelType(f.fields[fieldType].Value()),
		SSHHost:     f.fields[fieldSSHHost].Value(),
		LocalHost:   lh,
		LocalPort:   localPort,
		RemoteHost:  remoteHost,
		RemotePort:  remotePort,
		Profile:     f.fields[fieldProfile].Value(),
		AutoConnect: autoConnect,
	}
	return t.Validate()
}

// ClearError returns a copy of the form with the error message cleared.
func (f FormModel) ClearError() FormModel {
	f.errMsg = ""
	return f
}

// WithError returns a copy of the form with the error message set.
func (f FormModel) WithError(msg string) FormModel {
	f.errMsg = msg
	return f
}

// Render renders the form.
func (f FormModel) Render(width int) string {
	title := "Add Tunnel"
	if f.isEdit {
		title = "Edit Tunnel"
	}

	var sb strings.Builder
	sb.WriteString(styleTitle.Render(title) + "\n\n")

	for i := range numFields {
		label := fieldLabels[i]
		labelStyle := styleLabelNormal
		if i == f.focused {
			labelStyle = styleLabelFocused
		}

		sb.WriteString(labelStyle.Render(fmt.Sprintf("  %-28s", label)))
		sb.WriteString(f.fields[i].View())
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	if f.errMsg != "" {
		sb.WriteString(styleError.Render("  Error: "+f.errMsg) + "\n\n")
	}

	sb.WriteString(styleFormHint.Render(formHintText))

	return styleBorder.Width(width - 4).Render(sb.String())
}

package ui

import (
	"testing"

	"github.com/takaaki-s/tunnelman/internal/daemon"
)

func TestNewForm_Defaults(t *testing.T) {
	f := NewForm()
	if f.isEdit {
		t.Error("NewForm should not be edit mode")
	}
	if f.fields[fieldType].Value() != "local" {
		t.Errorf("expected default type 'local', got %q", f.fields[fieldType].Value())
	}
	if f.fields[fieldAutoConnect].Value() != "false" {
		t.Errorf("expected default auto_connect 'false', got %q", f.fields[fieldAutoConnect].Value())
	}
	if f.focused != 0 {
		t.Errorf("expected first field focused, got %d", f.focused)
	}
}

func TestNewEditForm_Prefill(t *testing.T) {
	t1 := daemon.TunnelInfo{
		ID: "abc-123", Name: "mytest", Type: "remote",
		SSHHost: "server1", LocalPort: 2222, RemotePort: 22,
		Profile: "dev", AutoConnect: true,
	}
	f := NewEditForm(t1)
	if !f.isEdit {
		t.Error("NewEditForm should be in edit mode")
	}
	if f.fields[fieldName].Value() != "mytest" {
		t.Errorf("expected name 'mytest', got %q", f.fields[fieldName].Value())
	}
	if f.fields[fieldType].Value() != "remote" {
		t.Errorf("expected type 'remote', got %q", f.fields[fieldType].Value())
	}
	if f.fields[fieldAutoConnect].Value() != "true" {
		t.Errorf("expected auto_connect 'true', got %q", f.fields[fieldAutoConnect].Value())
	}
}

func TestFocusNext(t *testing.T) {
	f := NewForm()
	if f.focused != 0 {
		t.Fatalf("expected focused=0, got %d", f.focused)
	}
	f, _ = f.FocusNext()
	if f.focused != 1 {
		t.Errorf("expected focused=1, got %d", f.focused)
	}
}

func TestFocusPrev(t *testing.T) {
	f := NewForm()
	f, _ = f.FocusNext() // move to field 1 properly
	f, _ = f.FocusPrev()
	if f.focused != 0 {
		t.Errorf("expected focused=0, got %d", f.focused)
	}
}

func TestFocusNext_WrapAround(t *testing.T) {
	f := NewForm()
	f.focused = numFields - 1
	f.fields[0].Blur()
	f.fields[numFields-1].Focus()
	f, _ = f.FocusNext()
	if f.focused != 0 {
		t.Errorf("expected wrap to 0, got %d", f.focused)
	}
}

func TestIsLastField(t *testing.T) {
	f := NewForm()
	f.focused = numFields - 1
	if !f.IsLastField() {
		t.Error("expected IsLastField() = true")
	}
	f.focused = 0
	if f.IsLastField() {
		t.Error("expected IsLastField() = false for first field")
	}
}

func TestToAddRequest_Valid(t *testing.T) {
	f := NewForm()
	f.fields[fieldName].SetValue("mytest")
	f.fields[fieldType].SetValue("local")
	f.fields[fieldSSHHost].SetValue("server1")
	f.fields[fieldLocalPort].SetValue("8080")
	f.fields[fieldRemoteHost].SetValue("127.0.0.1")
	f.fields[fieldRemotePort].SetValue("80")

	req, err := f.ToAddRequest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Name != "mytest" {
		t.Errorf("expected name 'mytest', got %q", req.Name)
	}
	if req.LocalPort != 8080 {
		t.Errorf("expected local_port=8080, got %d", req.LocalPort)
	}
}

func TestToAddRequest_Invalid_MissingName(t *testing.T) {
	f := NewForm()
	f.fields[fieldType].SetValue("local")
	f.fields[fieldSSHHost].SetValue("server1")
	f.fields[fieldLocalPort].SetValue("8080")
	f.fields[fieldRemotePort].SetValue("80")

	_, err := f.ToAddRequest()
	if err == nil {
		t.Error("expected validation error for missing name")
	}
}

func TestToAddRequest_Invalid_BadPort(t *testing.T) {
	f := NewForm()
	f.fields[fieldName].SetValue("mytest")
	f.fields[fieldType].SetValue("local")
	f.fields[fieldSSHHost].SetValue("server1")
	f.fields[fieldLocalPort].SetValue("notaport")

	_, err := f.ToAddRequest()
	if err == nil {
		t.Error("expected validation error for invalid port")
	}
}

func TestToAddRequest_Invalid_BadRemotePort(t *testing.T) {
	f := NewForm()
	f.fields[fieldName].SetValue("mytest")
	f.fields[fieldType].SetValue("local")
	f.fields[fieldSSHHost].SetValue("server1")
	f.fields[fieldLocalPort].SetValue("8080")
	f.fields[fieldRemotePort].SetValue("notaport")

	_, err := f.ToAddRequest()
	if err == nil {
		t.Error("expected validation error for invalid remote port")
	}
}

func TestToAddRequest_Invalid_EmptyLocalPort(t *testing.T) {
	f := NewForm()
	f.fields[fieldName].SetValue("mytest")
	f.fields[fieldType].SetValue("local")
	f.fields[fieldSSHHost].SetValue("server1")
	// localPort is empty

	_, err := f.ToAddRequest()
	if err == nil {
		t.Error("expected validation error for empty local port")
	}
}

func TestToEditRequest_Valid(t *testing.T) {
	t1 := daemon.TunnelInfo{
		ID: "abc-123", Name: "mytest", Type: "local",
		SSHHost: "server1", LocalPort: 8080, RemotePort: 80,
	}
	f := NewEditForm(t1)
	f.fields[fieldName].SetValue("updated")

	req, err := f.ToEditRequest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ID != "abc-123" {
		t.Errorf("expected ID 'abc-123', got %q", req.ID)
	}
	if *req.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", *req.Name)
	}
}

func TestToEditRequest_Invalid_MissingSSHHost(t *testing.T) {
	t1 := daemon.TunnelInfo{
		ID: "abc-123", Name: "mytest", Type: "local",
		SSHHost: "server1", LocalPort: 8080, RemotePort: 80,
	}
	f := NewEditForm(t1)
	f.fields[fieldSSHHost].SetValue("")

	_, err := f.ToEditRequest()
	if err == nil {
		t.Error("expected validation error for missing ssh host")
	}
}

func TestToAddRequest_DynamicType(t *testing.T) {
	f := NewForm()
	f.fields[fieldName].SetValue("socks")
	f.fields[fieldType].SetValue("dynamic")
	f.fields[fieldSSHHost].SetValue("server1")
	f.fields[fieldLocalPort].SetValue("1080")
	// RemoteHost and RemotePort empty — valid for dynamic

	_, err := f.ToAddRequest()
	if err != nil {
		t.Fatalf("unexpected error for dynamic type: %v", err)
	}
}

func TestToAddRequest_LocalType_EmptyRemoteHost(t *testing.T) {
	f := NewForm()
	f.fields[fieldName].SetValue("mytest")
	f.fields[fieldType].SetValue("local")
	f.fields[fieldSSHHost].SetValue("server1")
	f.fields[fieldLocalPort].SetValue("8080")
	f.fields[fieldRemotePort].SetValue("80")
	// RemoteHost intentionally left empty — should default to 127.0.0.1

	req, err := f.ToAddRequest()
	if err != nil {
		t.Fatalf("unexpected error for local type with empty remote host: %v", err)
	}
	if req.RemoteHost != "127.0.0.1" {
		t.Errorf("expected remoteHost '127.0.0.1', got %q", req.RemoteHost)
	}
}

func TestFocusPrev_WrapAround(t *testing.T) {
	f := NewForm()
	// focused=0, FocusPrev should wrap to last field
	f, _ = f.FocusPrev()
	if f.focused != numFields-1 {
		t.Errorf("expected wrap to %d, got %d", numFields-1, f.focused)
	}
}

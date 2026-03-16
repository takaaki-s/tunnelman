package ui

import (
	"strings"
	"testing"

	"github.com/takaaki-s/tunnelman/internal/daemon"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  int // expected rune length
	}{
		{"hello", 10, 10},     // padded
		{"hello", 5, 5},       // exact
		{"hello world", 5, 5}, // truncated (last rune is …)
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.width)
		if len([]rune(got)) != tt.want {
			t.Errorf("truncate(%q, %d): len=%d, want %d", tt.input, tt.width, len([]rune(got)), tt.want)
		}
	}
}

func TestTruncate_LongName(t *testing.T) {
	long := strings.Repeat("a", 30)
	got := truncate(long, colNameWidth)
	runes := []rune(got)
	if len(runes) != colNameWidth {
		t.Errorf("expected width %d, got %d", colNameWidth, len(runes))
	}
	// Last character should be ellipsis
	if string(runes[colNameWidth-1]) != "…" {
		t.Errorf("expected ellipsis at end, got %q", string(runes[colNameWidth-1]))
	}
}

func TestRenderTable_Empty(t *testing.T) {
	out := RenderTable(nil, 0, 0, 10)
	if out == "" {
		t.Error("expected non-empty output for empty table")
	}
}

func TestRenderTable_Single(t *testing.T) {
	tunnels := []daemon.TunnelInfo{
		{ID: "test-123", Name: "mytest", Type: "local", LocalPort: 8080, RemotePort: 80, SSHHost: "srv1", Status: "running"},
	}
	out := RenderTable(tunnels, 0, 0, 10)
	if !strings.Contains(out, "mytest") {
		t.Errorf("expected tunnel name in output, got: %s", out)
	}
}

func TestRenderTable_DynamicRemote(t *testing.T) {
	tunnels := []daemon.TunnelInfo{
		{ID: "dyn-1", Name: "socks", Type: "dynamic", LocalPort: 1080, Status: "running"},
	}
	out := RenderTable(tunnels, 0, 0, 10)
	if !strings.Contains(out, "-") {
		t.Errorf("expected '-' for dynamic remote port, got: %s", out)
	}
}

func TestStatusIcon(t *testing.T) {
	cases := map[string]string{
		"running":      "●",
		"stopped":      "○",
		"starting":     "◎",
		"reconnecting": "◎",
		"unhealthy":    "✕",
		"error":        "✕",
		"unknown":      "○",
	}
	for status, want := range cases {
		got := statusIcon(status)
		if got != want {
			t.Errorf("statusIcon(%q) = %q, want %q", status, got, want)
		}
	}
}

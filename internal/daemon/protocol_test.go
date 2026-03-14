package daemon

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name   string
		action string
		data   any
	}{
		{"add", ActionAdd, AddRequest{ID: "t1", Name: "web", Type: "local", SSHHost: "bastion", LocalPort: 8080, RemotePort: 80}},
		{"remove", ActionRemove, RemoveRequest{ID: "t1"}},
		{"start", ActionStart, StartRequest{ID: "t1"}},
		{"stop", ActionStop, StopRequest{ID: "t1"}},
		{"list", ActionList, ListRequest{Profile: "dev"}},
		{"status", ActionStatus, StatusRequest{ID: "t1"}},
		{"daemon_status", ActionDaemonStatus, nil},
		{"shutdown", ActionShutdown, nil},
		{"profile_create", ActionProfileCreate, ProfileCreateRequest{Name: "dev", Description: "Development"}},
		{"profile_rm", ActionProfileRemove, ProfileRemoveRequest{Name: "dev"}},
		{"profile_list", ActionProfileList, nil},
		{"import", ActionImport, ImportRequest{SSHConfigPath: "/home/user/.ssh/config", HostAlias: "bastion"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawData json.RawMessage
			if tt.data != nil {
				rawData, _ = json.Marshal(tt.data)
			}

			req := Request{Action: tt.action, Data: rawData}
			encoded, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var decoded Request
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			if decoded.Action != tt.action {
				t.Errorf("Action = %q, want %q", decoded.Action, tt.action)
			}
		})
	}
}

func TestAllActionConstants(t *testing.T) {
	actions := map[string]string{
		"add":            ActionAdd,
		"rm":             ActionRemove,
		"edit":           ActionEdit,
		"start":          ActionStart,
		"stop":           ActionStop,
		"start_all":      ActionStartAll,
		"stop_all":       ActionStopAll,
		"list":           ActionList,
		"status":         ActionStatus,
		"daemon_status":  ActionDaemonStatus,
		"shutdown":       ActionShutdown,
		"profile_list":   ActionProfileList,
		"profile_create": ActionProfileCreate,
		"profile_rm":     ActionProfileRemove,
		"import":         ActionImport,
	}

	for want, got := range actions {
		if got != want {
			t.Errorf("Action constant = %q, want %q", got, want)
		}
	}

	if len(actions) != 15 {
		t.Errorf("Expected 15 actions, got %d", len(actions))
	}
}

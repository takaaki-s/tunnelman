# tunnelman -- Developer Guide

## Project Overview

tunnelman is a CLI tool for managing SSH tunnels through a background daemon process, written in Go 1.24+.
The daemon manages SSH tunnel processes via Unix socket IPC, and the CLI communicates with it using a JSON-over-socket protocol.
All commands support `--json` output for LLM/script integration.

## Architecture

Layered architecture -- dependencies flow strictly downward. Violations are caught by `architecture_test.go`.

```
Layer 0 (leaf):   tunnel, config, sshconfig  ← no internal deps
Layer 1:          daemon → tunnel, config
Layer 2 (root):   cmd/tunnelman/cmd → config, daemon, sshconfig
```

### Package Dependency Rules

| Package          | Allowed internal imports |
|------------------|-------------------------|
| internal/tunnel  | NONE                    |
| internal/config  | NONE                    |
| internal/sshconfig | NONE                  |
| internal/daemon  | tunnel, config          |

### Package Responsibilities

- **cmd/tunnelman/cmd/** -- CLI layer (Cobra). Wires packages together. No business logic.
- **internal/tunnel/** -- Tunnel struct, TunnelType/Status, Validate, BuildSSHCommand, Clone. Profile struct.
- **internal/config/** -- YAML config parsing (LoadConfig/SaveConfig), StateManager (PID/tunnel state), XDG path helpers.
- **internal/sshconfig/** -- SSH config parser (~/.ssh/config). No internal deps.
- **internal/daemon/** -- Unix socket server, JSON-over-socket protocol, client, ProcessManager (Commander interface), HealthChecker, ReconnectManager.

## Build & Test

```bash
make build    # go build -o build/tunnelman ./cmd/tunnelman
make test     # go test -v -race ./...
make lint     # golangci-lint run ./...
make fmt      # Check formatting (non-zero exit if unformatted)
make clean    # Remove build artifacts
```

Always run `make test` and `make lint` before committing.

## Coding Conventions

- Standard library test style only -- no testify or other test frameworks.
- Table-driven tests where applicable.
- `t.TempDir()` for temp files in tests.
- TDD: write tests first, then implement, then refactor.
- Wrap errors with `fmt.Errorf("context: %w", err)`. No panics in library code.
- Commit messages: conventional commits (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`).
- mutex order: Server.mu → ProcessManager.mu (never reverse). HealthChecker.mu and ReconnectManager.mu are independent.
- macOS Unix socket path limit is 104 bytes -- keep test socket paths short.

## Do NOT

- Add internal dependencies to leaf packages (tunnel, config, sshconfig).
- Use testify or other test frameworks.
- Add `go:generate` or code generation.
- Modify the Unix socket protocol without updating both `daemon/server.go` and `daemon/client.go`.
- Add network-calling tests (no external SSH/HTTP calls in unit tests).
- Commit `.env` files or credentials.

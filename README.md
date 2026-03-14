# Tunnelman

A CLI tool for managing SSH tunnels through a background daemon process.

## Features

- **Daemon architecture**: Background daemon manages tunnels independently of your shell session
- **Multiple tunnel types**: Local (-L), Remote (-R), and Dynamic/SOCKS (-D) forwarding
- **Profile management**: Organize tunnels into profiles for different environments
- **SSH config import**: Import tunnel configurations from ~/.ssh/config
- **Health checking**: TCP-based health checks detect unhealthy tunnels
- **Auto-reconnect**: Automatic reconnection with exponential or fixed backoff
- **JSON output**: Machine-readable output for scripting with `--json`
- **XDG compliant**: Follows XDG Base Directory Specification for file storage

## Installation

```bash
go install github.com/takaaki-s/tunnelman/cmd/tunnelman@latest
```

### Build from source

```bash
git clone https://github.com/takaaki-s/tunnelman.git
cd tunnelman
make build
```

## Quick Start

```bash
# Start the daemon
tunnelman daemon start

# Add a tunnel
tunnelman add --id db --name "Database" --type local \
  --ssh-host bastion --local-port 5432 --remote-host db.internal --remote-port 5432

# Start the tunnel
tunnelman start db

# Check status
tunnelman status db

# List all tunnels
tunnelman list

# Stop and remove
tunnelman stop db
tunnelman rm db

# Stop the daemon
tunnelman daemon stop
```

## Commands

### Daemon management

```bash
tunnelman daemon start     # Start the daemon (background)
tunnelman daemon stop      # Stop the daemon
tunnelman daemon status    # Show daemon status
```

### Tunnel operations

```bash
tunnelman add [flags]      # Add a new tunnel
tunnelman rm <id>          # Remove a tunnel
tunnelman edit <id>        # Edit a tunnel
tunnelman start <id>       # Start a tunnel (or --all)
tunnelman stop <id>        # Stop a tunnel (or --all)
tunnelman list             # List tunnels
tunnelman status <id>      # Show tunnel status
```

### Profile management

```bash
tunnelman profile list                       # List profiles
tunnelman profile create <name> [--description ..]  # Create a profile
tunnelman profile rm <name>                  # Remove a profile
```

### SSH config import

```bash
tunnelman import --host <ssh-alias>          # Import forwards from SSH config
```

### Global flags

```
--json            Output in JSON format
--config <path>   Path to config file
--socket <path>   Path to daemon socket
```

## Configuration

Configuration is stored in YAML format:

- **Linux/macOS**: `~/.config/tunnelman/config.yaml`
- **Windows**: `%APPDATA%\tunnelman\config.yaml`

### Example

```yaml
tunnels:
  - id: db-tunnel
    name: Database Tunnel
    type: local
    ssh_host: bastion.example.com
    local_host: 127.0.0.1
    local_port: 5432
    remote_host: localhost
    remote_port: 5432
    profile: development
    auto_connect: false
  - id: socks-proxy
    name: SOCKS Proxy
    type: dynamic
    ssh_host: proxy.example.com
    local_host: 127.0.0.1
    local_port: 1080
profiles:
  - name: development
    description: Development environment
health_check:
  enabled: true
  interval_seconds: 30
  timeout_seconds: 5
  max_failures: 3
reconnect:
  enabled: true
  strategy: exponential
  initial_delay_seconds: 1
  max_delay_seconds: 300
  max_retries: 10
```

## Tunnel Types

### Local Forward (-L)

Forwards a local port to a remote destination through the SSH server.

```
Local:5432 → SSH Server → Remote DB:5432
```

### Remote Forward (-R)

Forwards a remote port on the SSH server to a local destination.

```
Remote:8080 → SSH Server → Local:3000
```

### Dynamic/SOCKS (-D)

Creates a SOCKS proxy on the local port.

```
Local:1080 → SOCKS Proxy → Any destination
```

## Development

### Requirements

- Go 1.24 or later
- SSH client installed

### Build and test

```bash
make build    # Build the binary
make test     # Run tests (with -race)
make lint     # Run linter
make fmt      # Check formatting
make clean    # Remove build artifacts
```

## License

MIT License - see LICENSE file for details

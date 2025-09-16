# Tunnelman

A powerful SSH tunnel manager with a Terminal User Interface (TUI) for easy management of SSH port forwarding connections.

## Features

- **Interactive TUI**: Manage SSH tunnels with an intuitive terminal interface built with tview
- **Multiple tunnel types**: Support for Local (-L), Remote (-R), and Dynamic/SOCKS (-D) forwarding
- **Profile management**: Organize tunnels into profiles for different environments
- **SSH Config Import**: Import tunnel configurations directly from ~/.ssh/config
- **Auto-connect**: Configure tunnels to start automatically on launch
- **Persistent connections**: Tunnels remain running even after closing the UI
- **Real-time status**: Monitor tunnel states with color-coded indicators
- **Search and filter**: Quickly find tunnels with search functionality
- **Cross-platform**: Works on Linux, macOS, and Windows
- **XDG compliant**: Follows XDG Base Directory Specification for file storage
- **SSH config integration**: Uses system SSH configuration (~/.ssh/config) for authentication

## Installation

### From source

```bash
go install github.com/takaaki-s/tunnelman/cmd/tunnelman@latest
```

### Build locally

```bash
git clone https://github.com/takaaki-s/tunnelman.git
cd tunnelman
make build
# Or manually:
# go build -o build/tunnelman cmd/tunnelman/main.go
```

## Usage

### Basic usage

```bash
# Start the TUI
tunnelman

# Start with a specific profile
tunnelman --profile development

# Auto-connect tunnels on startup and exit (headless mode)
tunnelman -auto

# Auto-connect specific profile tunnels on startup and exit
tunnelman -auto --profile production

# List available profiles
tunnelman --list-profiles

# Enable debug mode for verbose logging
tunnelman --debug

# Show version
tunnelman --version
```

### Keyboard shortcuts

#### Navigation
- `↑`/`k` - Move up
- `↓`/`j` - Move down
- `Tab` - Switch focus between panels
- `/` - Search tunnels
- `Esc` - Cancel search/Close dialog

#### Tunnel Operations
- `Enter` - Start/Stop selected tunnel
- `u` - Start selected tunnel
- `d` - Stop selected tunnel
- `c` - Create new tunnel
- `e` - Edit selected tunnel
- `r` - Remove (delete) selected tunnel
- `a` - Toggle auto-connect for selected tunnel
- `f` - Toggle forward/reverse mode (Local ↔ Remote)

#### Batch Operations
- `A` - Start all tunnels in current profile
- `X` - Stop all tunnels in current profile

#### Profile Management
- `g` - Switch profile
- `p` - Manage profiles (create/delete)
- `i` - Import tunnels from SSH config

#### Application
- `?` - Show help
- `q` - Quit (tunnels keep running)
- `Ctrl+C` - Force quit

## Configuration

Configuration files are stored according to the XDG Base Directory Specification:

- **Linux/macOS**: `~/.config/tunnelman/config.json`
- **Windows**: `%APPDATA%\tunnelman\config.json`

### Example configuration

```json
{
  "version": "1.0",
  "tunnels": [
    {
      "id": "db-tunnel",
      "name": "Database Tunnel",
      "type": "local",
      "ssh_host": "bastion.example.com",
      "local_host": "127.0.0.1",
      "local_port": 5432,
      "remote_host": "localhost",
      "remote_port": 5432,
      "profile": "development",
      "auto_connect": false
    },
    {
      "id": "web-tunnel",
      "name": "Web Server",
      "type": "local",
      "ssh_host": "web.example.com",
      "local_host": "127.0.0.1",
      "local_port": 8080,
      "remote_host": "localhost",
      "remote_port": 80,
      "profile": "production",
      "auto_connect": true
    },
    {
      "id": "socks-proxy",
      "name": "SOCKS Proxy",
      "type": "dynamic",
      "ssh_host": "proxy.example.com",
      "local_host": "127.0.0.1",
      "local_port": 1080,
      "profile": "default",
      "auto_connect": false
    }
  ],
  "profiles": [
    {
      "name": "default",
      "description": "Default profile"
    },
    {
      "name": "development",
      "description": "Development environment"
    },
    {
      "name": "production",
      "description": "Production environment"
    }
  ]
}
```

**Note**: SSH authentication is handled by your system's SSH configuration (`~/.ssh/config`). Configure your SSH hosts, users, ports, and keys there.

## Tunnel Types

### Local Forward (-L)
Forwards a local port to a remote destination through the SSH server.
```
Local:8080 → SSH Server → Remote:80
```
Use case: Access remote services as if they were local (e.g., databases, web servers)

### Remote Forward (-R)
Forwards a remote port on the SSH server to a local destination.
```
Remote:8080 → SSH Server → Local:3000
```
Use case: Expose local services to remote servers (e.g., webhooks, development servers)

**Note**: For external access, the SSH server must have `GatewayPorts` enabled in sshd_config

### Dynamic/SOCKS (-D)
Creates a SOCKS proxy on the local port.
```
Local:1080 → SOCKS Proxy → Any destination
```
Use case: Route traffic through SSH server as a proxy

## State Management

Running tunnel PIDs are stored in:
- **Linux/macOS**: `~/.local/state/tunnelman/pids.json`
- **Windows**: `%LocalAppData%\tunnelman\pids.json`

This allows Tunnelman to:
- Recover tunnel states after restart
- Clean up orphaned processes
- Keep tunnels running after UI exit

## SSH Configuration

Tunnelman relies on your system's SSH configuration for authentication. Configure your SSH settings in `~/.ssh/config`:

```ssh
Host bastion.example.com
    User myusername
    Port 22
    IdentityFile ~/.ssh/id_rsa

Host *.internal.example.com
    ProxyJump bastion.example.com
    User admin
```

### Importing from SSH Config

You can import tunnel configurations from your SSH config file:

1. Press `i` in the TUI to open the import dialog
2. Select the SSH host to import from
3. Choose or create a target profile
4. Tunnelman will automatically parse and import LocalForward, RemoteForward, and DynamicForward settings

Example SSH config with port forwarding:
```ssh
Host dev-server
    HostName dev.example.com
    User developer
    LocalForward 5432 localhost:5432
    LocalForward 8080 localhost:80
    RemoteForward 9000 localhost:3000
    DynamicForward 1080
```

This approach provides:
- Centralized SSH configuration
- Support for ProxyJump/ProxyCommand
- SSH agent integration
- Custom SSH options per host
- Easy migration from existing SSH setups

## Development

### Requirements
- Go 1.20 or later
- SSH client installed

### Building from source
```bash
# Clone the repository
git clone https://github.com/takaaki-s/tunnelman.git
cd tunnelman

# Build
make build

# Run tests
make test

# Install to $GOPATH/bin
make install

# Clean build artifacts
make clean
```

## Troubleshooting

### Tunnels not starting
- Check SSH connectivity: `ssh <host>` should work without password prompts
- Verify port availability: ensure local ports are not already in use
- Check logs with `--debug` flag for detailed error messages
- For Remote Forward, ensure SSH server has appropriate GatewayPorts setting

### Configuration not saving
- Verify write permissions for config directory
- Check disk space availability
- Run with `--debug` to see file operation errors

### Profile management issues
- Default profile cannot be deleted
- Profiles must have unique names
- Switching profiles only shows tunnels in that profile

### SSH Config import issues
- Ensure SSH config file has correct syntax
- Complex configurations may need manual adjustment
- Port forwarding directives must follow SSH config format

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Guidelines
1. Follow Go best practices and conventions
2. Add tests for new features
3. Update documentation as needed
4. Ensure all tests pass before submitting PR

## Author

Takaaki Sato

## Acknowledgments

Built with:
- [tview](https://github.com/rivo/tview) - Terminal UI library
- [tcell](https://github.com/gdamore/tcell) - Terminal handling library
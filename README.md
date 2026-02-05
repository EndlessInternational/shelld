# shelld

A persistent shell service that provides HTTP API access to a long-running shell session. Designed for LLMs that need to execute shell commands while maintaining state (environment variables, working directory, shell functions) across multiple requests.

**IMPORTANT** shelld includes no authentication of any kind. It MUST be used inside a private network or VPC that does not expose the service to the public internet. 

## Quick Start

```bash
# Build
go build -o bin/shelld ./cmd/shelld

# Create config
cat > config.toml << 'EOF'
[server]
port = 8080
EOF

# Run
./bin/shelld --config config.toml

# Use
curl -X POST -H "X-Shell-Key: my-key" http://localhost:8080/lock
curl -X POST -H "X-Shell-Key: my-key" -d "echo hello" http://localhost:8080/execute
curl -X POST -H "X-Shell-Key: my-key" http://localhost:8080/unlock
```

## Endpoints

| Method | Path | Key | Description |
|--------|------|------|-------------|
| POST | `/lock` | Yes | Lock shell to key |
| POST | `/execute` | Yes | Execute a command |
| POST | `/kill` | Yes | Interrupt current command (Ctrl+C) |
| POST | `/unlock` | Yes | Unlock the shell (recycle or shutdown) |
| GET | `/output` | Yes | Get output from last completed command |
| GET | `/state` | Yes | Get current shell state |
| GET | `/health` | No | Health check |

## Shell States

| State | Description |
|-------|-------------|
| `available` | Initial state. Unclock. Call `/lock` to lock shell to key. |
| `locked` | Shell locked to key. Waiting for commands. Call `/execute`. |
| `executing` | Shell executing a command. Wait or call `/kill`. |
| `unrecoverable` | Shell in error state. Call `/unlock` and restart. |

## Locking

The first request to `/lock` with an `X-Shell-Key` header locks the shell to that key. All subsequent requests must use the same key.

```bash
# First request locks the shell
curl -X POST -H "X-Shell-Key: abc123" http://localhost:8080/lock

# Same key works
curl -X POST -H "X-Shell-Key: abc123" -d "echo test" http://localhost:8080/execute

# Different key rejected (401)
curl -X POST -H "X-Shell-Key: wrong" -d "echo test" http://localhost:8080/execute
```

## Command Execution

### Basic Usage

```bash
curl -X POST -H "X-Shell-Key: $KEY" -d "echo hello" http://localhost:8080/execute
# Response: hello

# State persists
curl -X POST -H "X-Shell-Key: $KEY" -d "export FOO=bar" http://localhost:8080/execute
curl -X POST -H "X-Shell-Key: $KEY" -d 'echo $FOO' http://localhost:8080/execute
# Response: bar
```

### Command Timeout

Override the default timeout with the `X-Command-Timeout` header:

```bash
curl -X POST -H "X-Shell-Key: $KEY" -H "X-Command-Timeout: 30m" \
  -d "long-running-task" http://localhost:8080/execute
```

If a command times out:
- Returns `202 Accepted`
- Shell remains in `executing` state
- Command continues running in background
- Poll `/state` until `locked`, then call `/output` to get the result

### Multiline Commands and Heredocs

Commands are executed via base64 encoding to preserve structure:

```bash
# Heredocs work
curl -X POST -H "X-Shell-Key: $KEY" -d "cat > /tmp/test.txt << 'EOF'
hello world
EOF
cat /tmp/test.txt" http://localhost:8080/execute

# While loops work
curl -X POST -H "X-Shell-Key: $KEY" -d 'for i in 1 2 3; do
  echo "Number: $i"
done' http://localhost:8080/execute
```

## Configuration

```toml
[server]
port = 8080                    # HTTP port (default: 8080)
die_on_unlock = true           # If true, /unlock shuts down server

[shell]
command = "/bin/bash"          # Shell executable (default: /bin/bash)
working_directory = ""         # Initial directory (default: shelld's cwd)

[timeout]
command = "5m"                 # Default command timeout
command_maximum = "30m"        # Max allowed via header
idle = "30m"                   # Shutdown after inactivity
shutdown = "30s"               # Graceful shutdown timeout
kill = "5s"                    # SIGINT to SIGKILL grace period

[hooks]
shell = "/bin/sh"              # Shell for hooks
lock = ""                      # Run when shell is locked
unlock = ""                    # Run when shell is unlocked
```

### die_on_unlock

Controls what `/unlock` does:

- `die_on_unlock = true` (default): `/unlock` shuts down the server. Use for single-use containers.
- `die_on_unlock = false`: `/unlock` terminates the shell and clears the key lock, but keeps the server running. Returns to `available` state for the next client. Use for pooled containers.

Environment variables:
- `SHELLD_CONFIG` - Path to config file (alternative to `--config` flag)
- `SHELLD_KEY` - Set in hook commands to the current API key

## HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 202 | Command timed out (still running) |
| 400 | Bad request (empty command, invalid header) |
| 401 | Unauthorized (missing or invalid key) |
| 409 | Conflict (wrong state for operation) |
| 500 | Internal error |

## Building and Testing

```bash
# Build
go build -o bin/shelld ./cmd/shelld

# Unit tests
go test ./internal/...

# All tests (unit + integration)
./test.sh
```

## Docker

```bash
# Build Linux binary
GOOS=linux GOARCH=amd64 go build -o bin/shelld ./cmd/shelld

# Build and run container
docker build -t shelld .
docker run -p 8080:8080 shelld
```

## License

MIT License

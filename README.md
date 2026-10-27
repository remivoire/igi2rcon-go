# IGI2 RCON

A cross-platform rewrite and modern replacement of the official IGI2 multiplayer RCON console (`console.exe`) written in Go.

---

## Features & QoL Improvements

* **Interactive Readline Experience:**
  * **Command History:** Use Up/Down arrows to cycle through previous commands, persisted across sessions in `~/.igi2_rcon_history`.
  * **Tab Auto-Completion:** Auto-completes `sv`, `lo`, server subcommands, and client shortcuts.
  * **Safe Asynchronous Rendering:** Background server packets are cleanly printed above the prompt without tearing or erasing half-typed commands.
  * **Instant Screen Clearing:** Fast ANSI screen clearing (`clear` or `cls`) without spawning external shell processes.
* **Smart Client Shortcuts:**
  * Aliases for cumbersome game commands (e.g., `login <pass>`, `say <msg>`, `players`, `maps`, `map <id>`, `kick <id>`, `ban <id>`, `restart`, `next`). Raw commands (`sv ...`, `lo ...`) remain fully supported.
* **Auto-Login:** Pass `-password <pass>` on startup or enter `login <pass>` in console.
* **Single-Shot CLI Mode (`-c`):** Execute a command non-interactively and exit (ideal for cron jobs, Discord bots, and automated scripts).
* **Verbose Packet Debugging:** Toggle raw hex dumps and TX/RX logging with `-debug` or the `debug` command.
* **100% Cross-Platform:** Pure Go with zero Cgo dependencies. Compiles and runs identically on Windows, Linux, and macOS.

---

## Installation & Building

Requires Go 1.22+.

```bash
# Clone the repository
git clone https://github.com/remivoire/igi2rcon-go.git
cd igi2rcon-go

# Download dependencies
go mod tidy

# Build native binary
go build -o igi2_rcon igi2_rcon.go
```

### Cross-Compilation

```bash
# Build for Windows (.exe)
GOOS=windows GOARCH=amd64 go build -o igi2_rcon.exe igi2_rcon.go

# Build for macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o igi2_rcon_mac igi2_rcon.go
```

---

## Usage

### 1. Interactive Console

Connect directly with positional arguments or flags:

```bash
# Standard interactive launch
./igi2_rcon 127.0.0.1 26001

# With auto-login password
./igi2_rcon -host 127.0.0.1 -port 26001 -password secret

# With packet debugging enabled
./igi2_rcon -host 127.0.0.1 -debug
```

### 2. Single-Shot Command (Scripting / Bots)

Send a command, print the server response, and exit immediately:

```bash
./igi2_rcon -host 127.0.0.1 -port 26001 -pass secret -c "maps"
./igi2_rcon -host 127.0.0.1 -port 26001 -pass secret -c "say Server restarting in 5 minutes"
```

---

## Client Shortcuts Reference

| Command / Shortcut | Expands to | Description |
| :--- | :--- | :--- |
| `login <passwd>` | `<passwd>` | Authenticate with RCON password |
| `say <message>` | `lo announce("<msg>")` | Broadcast message to all players |
| `players` | `sv finger` | Display connected players and statuses |
| `maps` | `sv listmaps` | List available map IDs |
| `map <mapID>` | `sv gotomap <mapID>` | Change to specific map ID |
| `kick <playerID>` | `sv kick <playerID>` | Kick player by ID |
| `ban <playerID>` | `sv ban <playerID>` | Ban player by ID |
| `unban <num>` | `sv unban <num>` | Remove ban from list |
| `next` | `sv gotonext` | Switch to next map in rotation |
| `restart` | `sv restartmap` | Restart current map |
| `debug` | *(client)* | Toggle raw packet debug logging |
| `clear` / `cls` | *(client)* | Clear the terminal screen |
| `help` | *(client)* | Show all available server commands & shortcuts |
| `exit` / `quit` | *(client)* | Exit the application |

---

## Technical Details

The application utilizes UDP via Go's standard `net` package. Outgoing commands are prefixed with `/` as required by the IGI2 engine. Server responses are parsed by stripping the 24-byte engine header and null terminator.

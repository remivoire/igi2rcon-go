package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chzyer/readline"
)

type Command struct {
	Name string
	Desc string
}

var helpCommands = []Command{
	{`sv rconpass("<passwd>")`, "Change the remote control password"},
	{`sv rconpass`, "Display the remote control password"},
	{`sv finger`, "Displays player names, IPs, time online and RCON status"},
	{`sv kick <playerID>`, "Kick player by ID"},
	{`sv votekick <playerID>`, "Vote to kick player"},
	{`sv ban <playerID>`, "Ban player by ID"},
	{`sv listban`, "List banned IPs"},
	{`sv unban <list_num>`, "Remove IP from ban list"},
	{`sv suicide`, "Die and respawn"},
	{`sv restartmap`, "Restart current map"},
	{`sv listmaps`, "List available maps and IDs"},
	{`lo activatemap <mapID>`, "Add map to rotation"},
	{`lo deactivatemap <mapID>`, "Remove map from rotation"},
	{`sv gotonext`, "Load next map in cycle"},
	{`sv gotomap <mapID>`, "Change to specific map ID"},
	{`sv showvotes`, "Show active kick/map votes"},
	{`sv addmoney <amount>`, "Add money to all players"},
	{`sv monitor`, "Toggle network status info"},
	{`sv/lo ploss <amount>`, "Simulate packet loss"},
	{`sv/lo timeout <sec>`, "Set connection timeout"},
	{`lo bandwidth <num>`, "Adjust client bandwidth (0-1000)"},
	{`lo svname("<name>")`, "Set server name"},
	{`lo svpassword("<pw>")`, "Set server join password"},
	{`lo maxplayers <num>`, "Set max player count"},
	{`lo teamdamage <0|1>`, "Toggle friendly fire"},
	{`lo warmup <sec>`, "Set warmup period"},
	{`lo public <0|1>`, "Toggle master server listing"},
	{`lo autokick <sec>`, "Set idle kick timer"},
	{`lo announce("<msg>")`, "Broadcast message to all players"},
	{`lo pingmax <ms>`, "Set max allowed latency"},
	{`lo moneystart <amt>`, "Starting cash per map"},
	{`lo maprounds <num>`, "Map limit by rounds"},
	{`lo maptime <min>`, "Map limit by time"},
	{`lo playername("<name>")`, "Set local player name"},
}

var shortcutCommands = []Command{
	{`login <passwd>`, "Log into RCON (sends password directly)"},
	{`say <msg>`, `Broadcast message to all players (lo announce("<msg>"))`},
	{`players`, "Display connected players and statuses (sv finger)"},
	{`kick <playerID>`, "Kick player by ID (sv kick <playerID>)"},
	{`ban <playerID>`, "Ban player by ID (sv ban <playerID>)"},
	{`maps`, "List available maps and IDs (sv listmaps)"},
	{`map <mapID>`, "Change to specific map ID (sv gotomap <mapID>)"},
	{`next`, "Load next map in cycle (sv gotonext)"},
	{`restart`, "Restart current map (sv restartmap)"},
	{`debug`, "Toggle raw packet debug logging on/off"},
	{`clear / cls`, "Clear the terminal screen"},
	{`exit`, "Exit the console"},
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type IGI2Console struct {
	Host      string
	Port      int
	Password  string
	Conn      *net.UDPConn
	IsRunning bool
	Debug     bool

	outMu  sync.Mutex
	output io.Writer
}

func NewIGI2Console(host string, port int, password string, debug bool) (*IGI2Console, error) {
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return nil, err
	}

	return &IGI2Console{
		Host:      host,
		Port:      port,
		Password:  password,
		Conn:      conn,
		IsRunning: true,
		Debug:     debug,
		output:    os.Stdout,
	}, nil
}

func (c *IGI2Console) SetOutput(w io.Writer) {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	c.output = w
}

func (c *IGI2Console) Log(msg string) {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	if c.output != nil {
		fmt.Fprintln(c.output, msg)
	} else {
		fmt.Println(msg)
	}
}

func (c *IGI2Console) ReceiveLoop() {
	buffer := make([]byte, 4096)
	for c.IsRunning {
		c.Conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := c.Conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if c.IsRunning && c.Debug {
				c.Log(fmt.Sprintf("[Connection Error] %v", err))
			}
			continue
		}

		if c.Debug {
			headerEnd := minInt(n, 24)
			c.Log(fmt.Sprintf("[DEBUG RX] %d bytes | Header: %02x", n, buffer[:headerEnd]))
		}

		if n >= 24 {
			// Skip 24-byte header & trim null characters
			payload := buffer[24:n]
			nullIndex := bytes.IndexByte(payload, 0)
			if nullIndex != -1 {
				payload = payload[:nullIndex]
			}
			text := strings.TrimRight(string(payload), "\r\n")
			if text != "" {
				c.Log(text)
			}
		}
	}
}

func (c *IGI2Console) SendCommand(command string) {
	if command == "" {
		return
	}

	payload := fmt.Sprintf("/%s", command)
	if c.Debug {
		c.Log(fmt.Sprintf("[DEBUG TX] '%s' (%d bytes)", payload, len(payload)))
	}

	_, err := c.Conn.Write([]byte(payload))
	if err != nil {
		c.Log(fmt.Sprintf("[Error] Failed to send command: '%s'. Error: %v", payload, err))
	}
}

func (c *IGI2Console) DisplayHelp() {
	var sb strings.Builder
	sb.WriteString("\n--- IGI2 Client Shortcuts ---\n")
	for _, sc := range shortcutCommands {
		sb.WriteString(fmt.Sprintf("%-30s | %s\n", sc.Name, sc.Desc))
	}
	sb.WriteString("\n--- Raw IGI2 Server Commands ---\n")
	for _, cmd := range helpCommands {
		sb.WriteString(fmt.Sprintf("%-30s | %s\n", cmd.Name, cmd.Desc))
	}
	sb.WriteString("--------------------------------\n")
	c.Log(sb.String())
}

func (c *IGI2Console) ClearScreen() {
	// Standard cross-platform ANSI escape sequence (works on Linux, macOS, and modern Windows)
	c.Log("\033[2J\033[H")
}

func (c *IGI2Console) ExpandShortcut(input string) string {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)

	switch {
	case lower == "players":
		return "sv finger"
	case lower == "maps":
		return "sv listmaps"
	case lower == "next":
		return "sv gotonext"
	case lower == "restart":
		return "sv restartmap"
	case strings.HasPrefix(lower, "login "):
		pass := strings.TrimSpace(trimmed[6:])
		pass = strings.Trim(pass, `"'`)
		return pass
	case strings.HasPrefix(lower, "say "):
		msg := strings.TrimSpace(trimmed[4:])
		msg = strings.Trim(msg, `"'`)
		return fmt.Sprintf(`lo announce("%s")`, msg)
	case strings.HasPrefix(lower, "announce "):
		msg := strings.TrimSpace(trimmed[9:])
		msg = strings.Trim(msg, `"'`)
		return fmt.Sprintf(`lo announce("%s")`, msg)
	case strings.HasPrefix(lower, "kick "):
		id := strings.TrimSpace(trimmed[5:])
		return fmt.Sprintf("sv kick %s", id)
	case strings.HasPrefix(lower, "ban "):
		id := strings.TrimSpace(trimmed[4:])
		return fmt.Sprintf("sv ban %s", id)
	case strings.HasPrefix(lower, "unban "):
		id := strings.TrimSpace(trimmed[6:])
		return fmt.Sprintf("sv unban %s", id)
	case strings.HasPrefix(lower, "map "):
		id := strings.TrimSpace(trimmed[4:])
		return fmt.Sprintf("sv gotomap %s", id)
	default:
		return trimmed
	}
}

func buildCompleter() *readline.PrefixCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("sv",
			readline.PcItem("rconpass"),
			readline.PcItem("finger"),
			readline.PcItem("kick"),
			readline.PcItem("votekick"),
			readline.PcItem("ban"),
			readline.PcItem("listban"),
			readline.PcItem("unban"),
			readline.PcItem("suicide"),
			readline.PcItem("restartmap"),
			readline.PcItem("listmaps"),
			readline.PcItem("gotonext"),
			readline.PcItem("gotomap"),
			readline.PcItem("showvotes"),
			readline.PcItem("addmoney"),
			readline.PcItem("monitor"),
			readline.PcItem("ploss"),
			readline.PcItem("timeout"),
		),
		readline.PcItem("lo",
			readline.PcItem("activatemap"),
			readline.PcItem("deactivatemap"),
			readline.PcItem("ploss"),
			readline.PcItem("timeout"),
			readline.PcItem("bandwidth"),
			readline.PcItem("svname"),
			readline.PcItem("svpassword"),
			readline.PcItem("maxplayers"),
			readline.PcItem("teamdamage"),
			readline.PcItem("warmup"),
			readline.PcItem("public"),
			readline.PcItem("autokick"),
			readline.PcItem("announce"),
			readline.PcItem("pingmax"),
			readline.PcItem("moneystart"),
			readline.PcItem("maprounds"),
			readline.PcItem("maptime"),
			readline.PcItem("playername"),
		),
		readline.PcItem("login"),
		readline.PcItem("say"),
		readline.PcItem("announce"),
		readline.PcItem("players"),
		readline.PcItem("kick"),
		readline.PcItem("ban"),
		readline.PcItem("unban"),
		readline.PcItem("maps"),
		readline.PcItem("map"),
		readline.PcItem("next"),
		readline.PcItem("restart"),
		readline.PcItem("help"),
		readline.PcItem("clear"),
		readline.PcItem("cls"),
		readline.PcItem("debug"),
		readline.PcItem("exit"),
	)
}

func (c *IGI2Console) StartREPL() {
	historyFile := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		historyFile = filepath.Join(home, ".igi2_rcon_history")
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "IGI> ",
		HistoryFile:     historyFile,
		AutoComplete:    buildCompleter(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Failed to initialize readline: %v\n", err)
		return
	}
	defer rl.Close()

	// redirect console log output through readline's Stdout writer
	// ensures incoming server packets are cleanly drawn above the prompt without clobbering input
	c.SetOutput(rl.Stdout())

	c.Log("IGI 2 RCON Console")
	c.Log(fmt.Sprintf("Target: %s:%d", c.Host, c.Port))
	if c.Password != "" {
		c.Log("[*] Authenticating with server...")
		c.SendCommand(c.Password)
	} else {
		c.Log("Enter password (or 'login <password>') to access RCON.")
	}
	c.Log("Type 'help' to view commands and shortcuts, or Tab to autocomplete.")

	// bg listener goroutine
	go c.ReceiveLoop()

	//  graceful OS interrupt hhandling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		c.IsRunning = false
		rl.Close()
		c.Conn.Close()
		os.Exit(0)
	}()

	for c.IsRunning {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			}
			continue
		} else if err == io.EOF {
			break
		} else if err != nil {
			if c.IsRunning {
				c.Log(fmt.Sprintf("Readline error: %v", err))
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		cmdLower := strings.ToLower(line)
		if cmdLower == "exit" || cmdLower == "quit" {
			c.IsRunning = false
			break
		}

		if cmdLower == "help" {
			c.DisplayHelp()
			continue
		}

		if cmdLower == "clear" || cmdLower == "cls" {
			c.ClearScreen()
			continue
		}

		if cmdLower == "debug" {
			c.Debug = !c.Debug
			status := "OFF"
			if c.Debug {
				status = "ON"
			}
			c.Log(fmt.Sprintf("[*] Debug mode: %s", status))
			continue
		}

		// Expand convenient shortcuts (e.g., 'say hello', 'players', 'maps', etc.)
		expandedCmd := c.ExpandShortcut(line)
		c.SendCommand(expandedCmd)
	}

	c.Log("Exiting...")
	c.IsRunning = false
	c.Conn.Close()
	time.Sleep(100 * time.Millisecond)
}

func (c *IGI2Console) RunSingleCommand(command string, timeout time.Duration) {
	// password = provided? then auth
	if c.Password != "" {
		c.SendCommand(c.Password)
		time.Sleep(100 * time.Millisecond)
	}

	expandedCmd := c.ExpandShortcut(command)
	c.SendCommand(expandedCmd)

	// collect respnose until timeout
	deadline := time.Now().Add(timeout)
	c.Conn.SetReadDeadline(deadline)

	buffer := make([]byte, 4096)
	for {
		n, _, err := c.Conn.ReadFromUDP(buffer)
		if err != nil {
			break
		}

		if c.Debug {
			headerEnd := minInt(n, 24)
			fmt.Printf("[DEBUG RX] %d bytes | Header: %02x\n", n, buffer[:headerEnd])
		}

		if n >= 24 {
			payload := buffer[24:n]
			nullIndex := bytes.IndexByte(payload, 0)
			if nullIndex != -1 {
				payload = payload[:nullIndex]
			}
			text := strings.TrimRight(string(payload), "\r\n")
			if text != "" {
				fmt.Println(text)
			}
		}
	}
}

func printUsage() {
	fmt.Printf(`IGI 2 RCON Client

Usage:
  Interactive Mode:
    %s [flags] [ip] [port]
    %s 127.0.0.1 26001

  Single Command Mode:
    %s [flags] -c "<command>"

Flags:
  -host, -h        Server IP address or hostname
  -port, -p        Server UDP port (default: 26001)
  -password, -pass RCON password (auto-authenticates)
  -command, -c     Execute a single command non-interactively and exit
  -timeout, -t     Timeout in seconds for single command mode (default: 2)
  -debug, -d       Enable raw packet debug output
  -help            Show this help message
`, os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	var host string
	var port int
	var password string
	var command string
	var timeoutSec int
	var debug bool
	var showHelp bool

	flag.StringVar(&host, "host", "", "Server IP or hostname")
	flag.StringVar(&host, "h", "", "Server IP or hostname (shorthand)")
	flag.IntVar(&port, "port", 26001, "Server UDP port")
	flag.IntVar(&port, "p", 26001, "Server UDP port (shorthand)")
	flag.StringVar(&password, "password", "", "RCON password")
	flag.StringVar(&password, "pass", "", "RCON password (shorthand)")
	flag.StringVar(&command, "command", "", "Execute a single command and exit")
	flag.StringVar(&command, "c", "", "Execute a single command and exit (shorthand)")
	flag.IntVar(&timeoutSec, "timeout", 2, "Response timeout in seconds for single command")
	flag.IntVar(&timeoutSec, "t", 2, "Response timeout in seconds (shorthand)")
	flag.BoolVar(&debug, "debug", false, "Enable verbose packet debug output")
	flag.BoolVar(&debug, "d", false, "Enable verbose packet debug output (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help")

	flag.Usage = printUsage
	flag.Parse()

	if showHelp {
		printUsage()
		os.Exit(0)
	}

	// backwards compatibility: positional arg fallback
	posArgs := flag.Args()
	if host == "" && len(posArgs) > 0 {
		host = posArgs[0]
	}
	if len(posArgs) > 1 {
		if p, err := strconv.Atoi(posArgs[1]); err == nil {
			port = p
		}
	}

	if host == "" {
		printUsage()
		os.Exit(1)
	}

	console, err := NewIGI2Console(host, port, password, debug)
	if err != nil {
		fmt.Printf("Failed to initialize console: %v\n", err)
		os.Exit(1)
	}

	if command != "" {
		console.RunSingleCommand(command, time.Duration(timeoutSec)*time.Second)
		console.Conn.Close()
		return
	}

	console.StartREPL()
}

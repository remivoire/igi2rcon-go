package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Command struct {
	Name string
	Desc string
}


// help commands because some can't be bothered to read mp-commands.txt like the idiots they are
// this was a waste of time, but worth it i guess
var helpCommands = []Command{
	{`sv rcon("<passwd>")`, "Get access to remote control"},
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
	{`lo announce("<msg>")`, "Broadcast message to all"},
	{`lo pingmax <ms>`, "Set max allowed latency"},
	{`lo moneystart <amt>`, "Starting cash per map"},
	{`lo maprounds <num>`, "Map limit by rounds"},
	{`lo maptime <min>`, "Map limit by time"},
	{`lo playername("<name>")`, "Set local player name"},
	{"clear", "Clear the terminal screen"},
}

type IGI2Console struct {
	Host      string
	Port      int
	Conn      *net.UDPConn
	IsRunning bool
}

func NewIGI2Console(host string, port int) (*IGI2Console, error) {
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
		Conn:      conn,
		IsRunning: true,
	}, nil
}

func (c *IGI2Console) ReceiveLoop() {
	buffer := make([]byte, 4096)
	for c.IsRunning {
		c.Conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := c.Conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// timeout ok, loop
				continue
			}
			if c.IsRunning {
				fmt.Printf("\r\033[K[Connection Error] %v\nIGI> ", err)
			}
			continue
		}

		if n >= 24 {
			// skip header & trim nulls
			payload := buffer[24:n]
			nullIndex := bytes.IndexByte(payload, 0)
			if nullIndex != -1 {
				payload = payload[:nullIndex]
			}
			text := string(payload)

			text = strings.TrimSuffix(text, "\n")

			// refresh prompt
			fmt.Printf("\r\033[K%s\nIGI> ", text)
		}
	}
}

func (c *IGI2Console) SendCommand(command string) {
	if command == "" {
		return
	}

	payload := fmt.Sprintf("/%s", command)
	_, err := c.Conn.Write([]byte(payload))
	if err != nil {
		fmt.Printf("\nFailed to send command: '%s'. Error: %v\nIGI> ", payload, err)
	}
}

func (c *IGI2Console) DisplayHelp() {
	fmt.Printf("\n--- IGI2 RCON command reference ---\n")
	for _, cmd := range helpCommands {
		fmt.Printf("%-30s | %s\n", cmd.Name, cmd.Desc)
	}
	fmt.Printf("------------------------------------\n\n")
}

func (c *IGI2Console) ClearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func (c *IGI2Console) StartREPL() {
	fmt.Println("IGI 2 RCON")
	fmt.Printf("Target: %s:%d\n", c.Host, c.Port)
	fmt.Println("Enter password, then enter to access the server for RCON")
	fmt.Println("Type 'help' to see available server commands.")

	// bg listener
	go c.ReceiveLoop()

	reader := bufio.NewReader(os.Stdin)
	for c.IsRunning {
		fmt.Print("IGI> ")
		command, err := reader.ReadString('\n')
		if err != nil {
			c.IsRunning = false
			break
		}

		// trim whitespace
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		cmdLower := strings.ToLower(command)
		if cmdLower == "exit" {
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

		c.SendCommand(command)
	}

	fmt.Println("\nExiting...")
	c.Conn.Close()
	// give it a sec to close
	time.Sleep(500 * time.Millisecond)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s <ip> <port>\n", os.Args[0])
		os.Exit(1)
	}

	ip := os.Args[1]
	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Invalid port number")
		os.Exit(1)
	}

	console, err := NewIGI2Console(ip, port)
	if err != nil {
		fmt.Printf("Failed to initialize console: %v\n", err)
		os.Exit(1)
	}

	console.StartREPL()
}

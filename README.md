# IGI2 RCON

This is a rewrite/reimplementation of the official IGI2MP RCON application (console.exe) using Golang.

---

## Features

Nothing too fancy, operates exactly like the original console.exe application. Only new features include the inclusion of a help (`help`) command that shows all valid commands you can pass to the server.

---

## Installation

This tool requires the Go programming language.
1.  Clone the repo.
2.  Open a terminal in the cloned repo directory.
3.  Compile the application:
    ```bash
    go build igi2_rcon.go
    ```

---

## Usage

Start the console by providing the server IP address and the port number.

```bash
./igi2-rcon <ip_address> <port>
```
Example:
```bash
./igi2-rcon 127.0.0.1 26001
```

---

## Technical Details

The application utilizes the `net` package for UDP communication. It prepends a forward slash `/` to all outgoing strings to match what's required by the IGI2 server. Server responses are processed by skipping the 24-byte header and trimming null characters for clean text output.

## Tidbit

This is arguably the worst implementation of RCON I've ever seen in a game. This leads to - 
1. Dropped Packets: You ever noticed how some commands, despite being passed to the server, don't return a response? That is thanks to the very nature of UDP. The game server expects UDP over the same server port (e.g., 26001) for RCON. UDP lacks ACK (Acknowledgement) mechanisms, meaning if a packet gets lost in transit due to something minor, it's gone forever. Neither the client nor the server will ever know it was dropped.
2. Fighting for Bandwidth: Because RCON shares the exact same UDP port as gameplay, your admin commands are literally competing for network priority with serverside game logic. If there's anything heavy going on in the server the network buffer can fill up and silently trash your incoming RCON responses. 
3. No State / Sequencing: There are no packet IDs. If you spam three commands quickly the server might process them out of order or you might receive the response for command #3 before command #1. If a response gets dropped you have no way to know which command it was attached to.

I guess if it works, it works. I can't change much of that right now but I figured this'd be a bit faster and more multiplatform this way.
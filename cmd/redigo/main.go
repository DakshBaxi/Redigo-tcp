package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DakshBaxi/RediGo/internal/store"
)

const (
	defaultAddr = ":6380" //redis default is 6379; we use 6380 for safety
)

var (
	aofFile *os.File
	aofMu 	sync.Mutex
)

// CommandFunc is the function signature for a RediGo command.
type CommandFunc func(conn net.Conn, s *store.Store, args []string)

// Global command registry.
var commands = map[string]CommandFunc{
	"SET":    cmdSET,
	"SETEX":  cmdSETEX,
	"GET":    cmdGET,
	"DEL":    cmdDEL,
	"KEYS":   cmdKEYS,
	"PING":   cmdPING,
	"EXISTS": cmdEXISTS,
	"TTL":    cmdTTL,
	"EXPIRE": cmdEXPIRE,
	"INCR":   cmdINCR,
    "DECR":   cmdDECR,
	"CONFIG": cmdCONFIG,
	"INFO":   cmdINFO,
	"HELP":   cmdHELP,
	"QUIT":   cmdQUIT,
}

func main() {
	// Create the in-memory store instance shared by all connections.
	s := store.New()
// cleanupexpired
	go func() {
	for {
		time.Sleep(5 * time.Second)
		n := s.CleanupExpired()
		if n > 0 {
			log.Printf("Cleaned up %d expired keys\n", n)
		}
	}
}()

	// open aof file in append mode(create if not exists)
	f,err:=os.OpenFile("./redigo.aof",os.O_CREATE|os.O_APPEND|os.O_WRONLY,0644)
	if err != nil{
		log.Fatalf("failed to open AOF file: %v", err)
	}
	aofFile = f
	defer f.Close()

	// replay existing aof to restore state
	if err :=replayAOF(s,"./redigo.aof");err != nil {
        log.Printf("error replaying AOF: %v", err)
    }

	// Start listening on TCP port.
	log.Printf("RediGo listening on %s ...", defaultAddr)
	ln,err := net.Listen("tcp",defaultAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		log.Printf("new connection from %s", conn.RemoteAddr())

		// Handle each client in a separate goroutine.
		go handleConn(conn, s)
	}
}
func handleConn(conn net.Conn,s *store.Store){
	defer func() {
		log.Printf("closing connection from %s", conn.RemoteAddr())
		conn.Close()
	}()
		// Send a welcome banner (purely for dev friendliness).
	fmt.Fprintf(conn, "+OK RediGo Simple Text Server\r\n")
	fmt.Fprintf(conn, "Supports simple text commands.\r\n")
	fmt.Fprintf(conn, "Type HELP for commands.\r\n")

	reader := bufio.NewScanner(conn)
	for {
		// Prompt
		fmt.Fprint(conn,"> ")
			if !reader.Scan() {
			// Client closed or error
			if err := reader.Err(); err != nil {
				log.Printf("read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
			line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
			// Split on spaces for now: CMD key value
		parts := strings.Fields(line)
		cmd := strings.ToUpper(parts[0])
		args := parts[1:]
				// Look up command handler.
		handler, ok := commands[cmd]
		if !ok {
			// Clean error: don’t dump weird whitespace
			fmt.Fprintf(conn, "-ERR unknown command '%s'\r\n", cmd)
			continue
		}

		// Execute handler
		handler(conn, s, args)
			// Special: QUIT closes the connection from inside handler.
		if cmd == "QUIT" {
			return
		}
	}
}




package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/DakshBaxi/RediGo/internal/store"
)

const defaultPrimary = "localhost:6380"

func main() {
	primaryAddr := defaultPrimary
	if len(os.Args) > 1 {
		primaryAddr = os.Args[1]
	}

	s := store.New()
		// Simple periodic sync loop
	go func() {
		for {
			if err := syncOnce(primaryAddr, s); err != nil {
				log.Printf("sync error: %v", err)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	// Start a read-only server for clients on a different port, e.g. 6381
	addr := ":6381"
	log.Printf("RediGo replica listening on %s (primary=%s)...", addr, primaryAddr)
	ln, err := net.Listen("tcp", addr)
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
		log.Printf("new client connection from %s", conn.RemoteAddr())
		go handleReplicaClient(conn, s)
	}
}

func syncOnce(primaryAddr string, s *store.Store) error {
	log.Printf("sync: connecting to primary %s ...", primaryAddr)
	conn, err := net.Dial("tcp", primaryAddr)
	if err != nil {
		return fmt.Errorf("dial primary: %w", err)
	}
	defer conn.Close()

	// Send DUMPALL
	fmt.Fprintf(conn, "DUMPALL\r\n")

	reader := bufio.NewReader(conn)

	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read from primary: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "." {
			break
		}
		if line == "" {
			continue
		}
		// Ignore welcome banners / prompts from primary
		if strings.HasPrefix(line, "+OK") || strings.HasPrefix(line, "Supports ") || strings.HasPrefix(line, "Type HELP") || line == ">" {
			continue
		}
		lines = append(lines, line)
	}

	// Apply snapshot to local store
	log.Printf("sync: received %d commands", len(lines))

	// For simplicity, we clear local store by reinitializing it.
	// (You could add a Reset() method instead.)
	newStore := store.New()
	for _, cmdLine := range lines {
		applySnapshotCommand(newStore, cmdLine)
	}

	// Swap: we don't have a nice atomic swap on store pointer,
	// so in real design you'd wrap Store with another layer.
	// For this MVP, we just copy over map content.
	replaceStoreData(s, newStore)

	log.Printf("sync: applied snapshot")
	return nil
}

// applySnapshotCommand parses a single replay line like: "SET k v", "SETEX k ttl v", "RPUSH k v1 v2"
func applySnapshotCommand(s *store.Store, line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}
	cmd := strings.ToUpper(parts[0])
	args := parts[1:]

	switch cmd {
	case "SET":
		if len(args) < 2 {
			return
		}
		key := args[0]
		value := strings.Join(args[1:], " ")
		s.Set(key, value)
	case "SETEX":
		if len(args) < 3 {
			return
		}
		key := args[0]
		ttlStr := args[1]
		value := strings.Join(args[2:], " ")
		// TTL will be approx, but ok for learning
		ttl, err := parseInt64(ttlStr)
		if err != nil {
			return
		}
		s.Setwithttl(key, value, ttl)
	}
}

func parseInt64(sval string) (int64, error) {
	var n int64
	_, err := fmt.Sscan(sval, &n)
	return n, err
}

// replaceStoreData copies contents from src to dst (naive but fine for now).
func replaceStoreData(dst, src *store.Store) {
	// This is a bit hacky because Store fields are private.
	// For learning, we can just re-dump from src into dst.
	cmds := src.DumpCommands()
	for _, line := range cmds {
		applySnapshotCommand(dst, line)
	}
}
// handleReplicaClient: like primary, but READ ONLY.
func handleReplicaClient(conn net.Conn, s *store.Store) {
	defer conn.Close()
	fmt.Fprintf(conn, "+OK RediGo Replica (read-only)\r\n")

	reader := bufio.NewScanner(conn)
	for {
		fmt.Fprint(conn, "> ")
		if !reader.Scan() {
			return
		}
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := strings.ToUpper(parts[0])
		args := parts[1:]

		switch cmd {
		case "GET":
			// reuse same logic but only for reads
			val, ok := s.Get(args[0])
			if ok {
				fmt.Fprintf(conn, "\"%s\"\r\n", val)
			} else {
				fmt.Fprintf(conn, "(nil)\r\n")
			}
		case "INFO":
			stats := s.Stats()
			fmt.Fprintf(conn, "# Replica\r\n")
			fmt.Fprintf(conn, "keys:%d\r\n", stats.Keys)
			fmt.Fprintf(conn, "max_keys:%d\r\n", stats.MaxKeys)
			fmt.Fprintf(conn, "evictions:%d\r\n", stats.Evictions)
		case "QUIT":
			fmt.Fprintf(conn, "+OK bye\r\n")
			return
		default:
			fmt.Fprintf(conn, "-ERR READONLY replica: only GET/INFO/QUIT allowed for now\r\n")
		}
	}
}
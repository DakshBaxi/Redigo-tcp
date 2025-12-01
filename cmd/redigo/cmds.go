package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/DakshBaxi/RediGo/internal/store"
)

func cmdSET(conn net.Conn, s *store.Store, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(conn, "-ERR SET requires key and value\r\n")
		return
	}
	key := args[0]
	value := strings.Join(args[1:], " ")
	s.Set(key, value)
	appendAOF("SET", key, value)

	fmt.Fprintf(conn, "+OK\r\n")
}

func cmdSETEX(conn net.Conn, s *store.Store, args []string) {
	// setexx key ttl value
	if len(args) < 3 {
		fmt.Fprintf(conn, "-ERR SETEX requires key, ttl, value\r\n")
		return
	}
	key := args[0]
	ttlStr := args[1]
	ttl, err := strconv.ParseInt(ttlStr, 10, 64)
	if err != nil || ttl <= 0 {
		fmt.Fprintf(conn, "-ERR invalid ttl '%s'\r\n", ttlStr)
		return
	}
	value := strings.Join(args[2:], " ")
	s.Setwithttl(key, value, ttl)
	appendAOF("SETEX", key, ttlStr, value)
	fmt.Fprintf(conn, "+OK\r\n")
}

func cmdTTL(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(conn, "-ERR TTL requires key\r\n")
		return
	}
	key := args[0]
	ttl := s.TTL(key)
	// Redis semantics:
	// -2: key does not exist
	// -1: exists, no ttl
	fmt.Fprintf(conn, ":%d\r\n", ttl)
}

func cmdGET(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(conn, "-ERR GET requires key\r\n")
		return
	}
	key := args[0]
	if v, ok := s.Get(key); ok {
		fmt.Fprintf(conn, "\"%s\"\r\n", v)
	} else {
		fmt.Fprintf(conn, "(nil)\r\n")
	}
}

func cmdDEL(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(conn, "-ERR DEL requires key\r\n")
		return
	}
	key := args[0]
	if s.Del(key) {
		appendAOF("DEL", key)
		fmt.Fprintf(conn, ":1\r\n")
	} else {
		fmt.Fprintf(conn, ":0\r\n")
	}
}

func cmdKEYS(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 0 {
		fmt.Fprintf(conn, "-ERR KEYS does not take arguments\r\n")
		return
	}
	keys := s.Keys()
	if len(keys) == 0 {
		fmt.Fprintf(conn, "(empty)\r\n")
		return
	}
	for _, k := range keys {
		fmt.Fprintf(conn, "%s\r\n", k)
	}
}

func cmdPING(conn net.Conn, _ *store.Store, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(conn, "PONG\r\n")
		return
	}
	// If a message is passed, echo it (Redis-like)
	msg := strings.Join(args, " ")
	fmt.Fprintf(conn, "%s\r\n", msg)
}

func cmdEXISTS(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(conn, "-ERR EXISTS requires key\r\n")
		return
	}
	key := args[0]
	if _, ok := s.Get(key); ok {
		fmt.Fprintf(conn, ":1\r\n")
	} else {
		fmt.Fprintf(conn, ":0\r\n")
	}
}

func cmdHELP(conn net.Conn, _ *store.Store, args []string) {
	if len(args) != 0 {
		fmt.Fprintf(conn, "-ERR HELP does not take arguments\r\n")
		return
	}
	fmt.Fprintf(conn, "%s\r\n", store.HelpText())
}

func cmdQUIT(conn net.Conn, _ *store.Store, args []string) {
	if len(args) != 0 {
		fmt.Fprintf(conn, "-ERR QUIT does not take arguments\r\n")
		return
	}
	fmt.Fprintf(conn, "+OK bye\r\n")
}

func cmdEXPIRE(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 2 {
		fmt.Fprintf(conn, "there should be key and ttl\r\n")
		return
	}
	key := args[0]
	ttlStr := args[1]
	ttl, err := strconv.ParseInt(ttlStr, 10, 64)
	if err != nil || ttl <= 0 {
		fmt.Fprintf(conn, "-ERR invalid ttl '%s'\r\n", ttlStr)
		return
	}
	if ok := s.Expires(key, ttl); ok {
		appendAOF("EXPIRE", key, ttlStr)
		fmt.Fprintf(conn, "+OK\r\n")
	}
}

func cmdINCR(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(conn, "-ERR INCR requires key\r\n")
		return
	}
	key := args[0]

	// Get current value
	val, ok := s.Get(key)
	var num int64
	var err error

	if !ok {
		// New counter → treat as 0
		num = 1 // Because INCR increments once
		s.Set(key, "1")
		appendAOF("SET", key, "1")
		fmt.Fprintf(conn, ":%d\r\n", num)
		return
	} else {
		num, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			fmt.Fprintf(conn, "-ERR value is not an integer or out of range\r\n")
			return
		}
	}

	num++ // increment

	newVal := strconv.FormatInt(num, 10)
	s.Set(key, newVal)
	appendAOF("SET", key, newVal)

	// Redis returns the new value as integer reply
	fmt.Fprintf(conn, ":%d\r\n", num)
}

func cmdDECR(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 1 {
		fmt.Fprintf(conn, "-ERR DECR requires key\r\n")
		return
	}
	key := args[0]

	val, ok := s.Get(key)
	var num int64
	var err error

	if !ok {
		num = 0
	} else {
		num, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			fmt.Fprintf(conn, "-ERR value is not an integer or out of range\r\n")
			return
		}
	}

	num-- // decrement

	newVal := strconv.FormatInt(num, 10)
	s.Set(key, newVal)
	appendAOF("SET", key, newVal)

	fmt.Fprintf(conn, ":%d\r\n", num)
}


func cmdCONFIG(conn net.Conn, s *store.Store, args []string) {
	// Very simple: CONFIG MAXKEYS <n>
	if len(args) != 2 {
		fmt.Fprintf(conn, "-ERR CONFIG usage: CONFIG MAXKEYS <n>\r\n")
		return
	}
	sub := strings.ToUpper(args[0])
	if sub != "MAXKEYS" {
		fmt.Fprintf(conn, "-ERR CONFIG only supports MAXKEYS for now\r\n")
		return
	}
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 0 {
		fmt.Fprintf(conn, "-ERR invalid MAXKEYS value '%s'\r\n", args[1])
		return
	}
	s.SetMaxKeys(n)
	fmt.Fprintf(conn, "+OK\r\n")
}

func cmdINFO(conn net.Conn, s *store.Store, args []string) {
	if len(args) != 0 {
		fmt.Fprintf(conn, "-ERR INFO does not take arguments\r\n")
		return
	}
	stats := s.Stats()
	// Simple text output; could be nicer, but this is good for now.
	fmt.Fprintf(conn, "# Server\r\n")
	fmt.Fprintf(conn, "keys:%d\r\n", stats.Keys)
	fmt.Fprintf(conn, "max_keys:%d\r\n", stats.MaxKeys)
	fmt.Fprintf(conn, "evictions:%d\r\n", stats.Evictions)
	fmt.Fprintf(conn, "reads:%d\r\n", stats.Reads)
	fmt.Fprintf(conn, "writes:%d\r\n", stats.Writes)
}

package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/DakshBaxi/RediGo/internal/store"
)

// appendAOF("SET", key, value...)
// appendAOF("SETEX", key, ttl, value...)
// appendAOF("DEL", key)
// appendAOF("EXPIRE", key, ttl)
func appendAOF(parts ...string) {
	if aofFile == nil {
		return
	}
	line := strings.Join(parts, " ") + "\n"
	aofMu.Lock()
	defer aofMu.Unlock()

	if _, err := aofFile.WriteString(line); err != nil {
		log.Printf("AOF write error: %v", err)
	}
}

func replayAOF(s *store.Store,path string) error{
	f,err := os.Open(path)
	if err!=nil{
		   if os.IsNotExist(err) {
            return nil // nothing to replay yet
        }
        return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan(){
		line:=strings.TrimSpace(scanner.Text())
		if line ==""{
			continue
		}
		parts := strings.Fields(line)
		cmd := strings.ToUpper(parts[0])
		args := parts[1:]
     switch cmd {
        case "SET":
            if len(args) < 2 {
                continue
            }
            key := args[0]
            value := strings.Join(args[1:], " ")
            s.Set(key, value)

        case "SETEX":
            if len(args) < 3 {
                continue
            }
            key := args[0]
            ttlStr := args[1]
            ttl, err := strconv.ParseInt(ttlStr, 10, 64)
            if err != nil {
                continue
            }
            value := strings.Join(args[2:], " ")
            s.Setwithttl(key, value, ttl)

        case "DEL":
            if len(args) != 1 {
                continue
            }
            s.Del(args[0])

        case "EXPIRE":
            if len(args) != 2 {
                continue
            }
            key := args[0]
            ttlStr := args[1]
            ttl, err := strconv.ParseInt(ttlStr, 10, 64)
            if err != nil {
                continue
            }
            s.Expires(key, ttl)
        }
    }
    return scanner.Err()
}
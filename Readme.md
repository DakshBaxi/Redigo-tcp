# RediGo - Distributed Redis-like In-Memory Data Store

🚀 **RediGo** is a Redis-inspired in-memory key-value store built completely from scratch in Golang. It supports networking, TTL expiration, durable storage via AOF, list structures, and even distributed replication (Primary → Replica).

This project demonstrates real distributed database concepts and backend system design.

---

## ✨ Features

| Feature                                            | Status |
| -------------------------------------------------- | :----: |
| TCP command server                                 |    ✅   |
| String KV commands (`SET`, `GET`, `DEL`)           |    ✅   |
| TTL expiration (`EXPIRE`, `TTL`, `SETEX`)          |    ✅   |
| Integer operations (`INCR`, `DECR`)                |    ✅   |
| Persistence (AOF replay)                           |    ✅   |
| Max key eviction strategy                          |    ✅   |
| Distributed Replication (Primary + Replica)        |   🚀   |
| Stats (`INFO`)                                     |   ✔️   |

Upcoming:

* Streaming replication
* Sharding & multi-primary clusters
* RESP protocol support
* RDB snapshots

---

## 🧠 Architecture

```
              +------------------------+
              |       Primary DB       |
              |   (Writable Node)      |
              |                        |
Clients --->  |  - AOF persistence     |
(read/write)  |  - TTL + Eviction      |
              +-----------+------------+
                          |
                          | Periodic Sync (DUMPALL)
                          v
              +------------------------+
              |       Replica DB       |
              |   (Read-Only Node)     |
        Clients (read) ---> GET / INFO |
              +------------------------+
```

### Consistency Model

> **Eventual consistency** with periodic snapshot-based pull replication.

---

## 🚀 Getting Started

### Run Primary Server

```bash
go run ./cmd/redigo
```

Primary at: `localhost:6380`
﻿# RediGo — Simple Redis-like TCP Key-Value Server

RediGo is a compact, educational Redis-like key-value server written in Go. It implements a tiny text-based protocol over TCP, supports basic commands (SET, GET, TTL, INCR, DECR, DEL, etc.), a simple Append-Only File (AOF) persistence, and a minimal read-only replica that synchronizes periodically using a DUMPALL snapshot.

This repository is intended for learning and experimentation — the implementation favors clarity and simplicity over production readiness.

---

**Table of Contents**

- Project overview
- Features
- Protocol & supported commands
- Persistence and replication
- Design and internals
- Build & run
- Usage examples
- Development notes
- TODO / ideas
- License

---

**Project overview**

RediGo implements a tiny TCP-based server with a simple textual protocol inspired by Redis. It provides an in-memory key-value store with optional TTLs, basic statistics, and command-line examples to run both a primary and a read-only replica.

Key files / folders:

- `cmd/redigo/` — primary server: listener, command dispatch, AOF replay and append, background expiry cleanup.
- `cmd/redigo-replica/` — replica program: periodically connects to primary, requests a DUMPALL snapshot and applies it locally, and serves read-only clients.
- `internal/store/` — in-memory `Store` implementation: `Entry`, TTL handling, stats, DumpCommands for snapshot.
- `redigo.aof` — append-only file used by the primary to persist mutating commands (created by the server when started).

---

**Features**

- Text-based TCP protocol (human-friendly; useful for learning and debugging).
- Basic commands: `SET`, `SETEX` (set with TTL), `GET`, `DEL`, `KEYS`, `EXISTS`, `TTL`, `EXPIRE`, `INCR`, `DECR`, `CONFIG MAXKEYS`, `INFO`, `DUMPALL`, `HELP`, `QUIT`, `PING`.
- AOF persistence: append-only log of mutating commands that is replayed on startup.
- Background expiry cleanup: expired keys are removed periodically (every 5s by default).
- Read-only replica: `cmd/redigo-replica` connects to a primary, requests `DUMPALL`, and keeps a local snapshot refreshed periodically.

---

**Protocol & Supported Commands**

All commands are plain text lines with arguments separated by spaces. The server responds with simple text replies (a tiny subset of Redis reply styles are emulated):

- `SET key value` — sets `key` to `value` (no TTL). Replies `+OK`.
- `SETEX key ttl value` — sets `key` to `value` with TTL in seconds. Replies `+OK`.
- `GET key` — returns the value in quotes if present (e.g. `"value"`), or `(nil)` if absent.
- `DEL key` — deletes key; replies `:1` if deleted or `:0` if not found.
- `KEYS` — lists all keys (one per line) or `(empty)` if none.
- `EXISTS key` — `:1` if exists, `:0` otherwise.
- `TTL key` — returns remaining TTL in seconds as integer reply. Semantics:
      - `-2` — key does not exist or is expired
      - `-1` — key exists and has no TTL
- `EXPIRE key ttl` — set new TTL on an existing key; replies `+OK` on success.
- `INCR key` — increment integer value (initializes a missing key to `1` and returns `:1`). Returns error if value is not integer.
- `DECR key` — decrement integer value (initializes missing key to `0` then decrements) and replies with integer.
- `CONFIG MAXKEYS n` — set a soft maximum for the number of keys (0 = unlimited).
- `INFO` — returns simple stats (keys, max_keys, evictions, reads, writes).
- `DUMPALL` — primary produces a simple reconstruction snapshot: one textual command per line (`SET` or `SETEX`), terminated by a single `.` on its own line. Replica uses this to sync.
- `HELP` — prints a help message.
- `QUIT` — closes the connection.
- `PING [msg]` — replies `PONG` if no arg, otherwise echoes message.

Note: Replies intentionally mimic Redis string/integer patterns for familiarity; this is a learning project, not a full Redis implementation.

---

**Persistence (AOF)**

The primary server opens `./redigo.aof` in append mode and writes mutating commands (e.g. `SET`, `SETEX`, `DEL`, `EXPIRE`) as they occur. On startup the primary calls `replayAOF(...)` to reapply logged commands and restore the in-memory state. The AOF is a simple text log (not a Redis RDB/AOF encoding).

Files and behavior:

- `redigo.aof` — located in the repository root (or current working directory when running the server). The server creates it if missing.
- Because the AOF format is plain textual commands, it is portable and human-readable, but not optimized for performance or atomicity.

---

**Replication (read-only replica)**

`cmd/redigo-replica` implements a basic periodic sync:

- Connects to the primary (default `localhost:6380`), sends `DUMPALL` and reads the snapshot lines until a `.` terminator.
- Applies `SET` and `SETEX` lines to a new `Store` and then replaces the local store by copying snapshot data.
- Serves clients on port `:6381` (read-only): supports `GET`, `INFO`, and `QUIT`; other commands result in a `-ERR READONLY replica` reply.

Run the replica with an alternate primary address: `go run ./cmd/redigo-replica localhost:6380`.

---

**Design and internals**

Store (in `internal/store/store.go`):

- `Entry` struct: stores `Value string`, `ExpiresAt int64` (unix seconds; `0` = no expiry), and `LastAccess int64`.
- `Store` struct: contains a `map[string]Entry` protected by a `sync.RWMutex`, and statistics fields (`maxKeys`, `evictions`, `reads`, `writes`).
- TTL semantics: `TTL` returns `-1` for present keys without TTL, `-2` if key missing or expired (matching Redis-like conventions used in this project).
- `Set`, `Setwithttl`, `Get`, `Del`, `Expires`, `CleanupExpired`, `DumpCommands`, `Keys`, `Stats`, `SetMaxKeys` are implemented to provide the required API.
- Background expiry: primary server spawns a goroutine that calls `CleanupExpired()` every 5 seconds.

Concurrency notes:

- `Store` uses `sync.RWMutex` to allow concurrent readers and exclusive writers. The current implementation updates `LastAccess` and stores it back under the same lock.
- The replica applies snapshots by building a fresh `Store` and then copying the snapshot commands into the active store (a simple, safe approach for this MVP).

---

**Build & Run**

Prerequisites: Go 1.20+ (module aware). From the project root:

Start the primary server (default port `:6380`):

```bash
go run ./cmd/redigo
```

Start the replica (default connects to `localhost:6380` and serves on `:6381`):

```bash
go run ./cmd/redigo-replica
```

To specify a different primary for the replica:

```bash
go run ./cmd/redigo-replica other-host:6380
```

Notes: each invocation runs the program in the foreground. The primary will create `redigo.aof` in the working directory.

---

**Usage examples (connect with netcat / telnet)**

Open a connection to the primary:

```bash
nc localhost 6380
# or
telnet localhost 6380
```

Example session (user input shown after prompt `> `):

> SET mykey hello

+OK

> GET mykey

"hello"

> INCR counter

:1

> INCR counter

:2

> SETEX short 10 temp

+OK

> TTL short

:9   # approximate seconds remaining

> DUMPALL

SET mykey hello
SETEX short 9 temp
SET counter 2
.

On replica (read-only) the supported client commands are `GET`, `INFO`, and `QUIT`.

---

**Development notes**

- Command handlers are in `cmd/redigo/cmds.go`. Each handler writes replies directly to the `net.Conn` and calls `appendAOF(...)` (for mutating commands) to persist actions.
- Server main loop is in `cmd/redigo/main.go`: starts listener, replays AOF on startup, spawns expiry cleanup goroutine.
- Replica logic is in `cmd/redigo-replica/main.go`.
- Store implementation is in `internal/store/store.go` and contains helpers used by both primary and replica.

If you change the AOF format, update the `replayAOF` implementation (primary) and replica snapshot parsing if needed.

---

**Testing & debugging**

- There are no automated tests included in the repository. You can manually test behavior by running the server and interacting via `nc` / `telnet`.
- Check the AOF file (`redigo.aof`) for a record of mutating operations.

---

**TODO / Ideas**

- Add proper RESP protocol support for full Redis compatibility.
- Add unit tests for `internal/store` behavior (TTL, eviction, concurrency).
- Implement a more robust AOF rewrite / compaction mechanism.
- Support more data types (lists, hashes) and more commands.
- Make replica sync incremental (partial replication) instead of full DUMPALL.

---

**Contributing**

Contributions are welcome — open an issue or a PR. Keep changes focused, add tests for new behavior, and document any protocol changes clearly.

---



Enjoy exploring RediGo! If you'd like, I can also add unit tests, a CI workflow, or switch the protocol to RESP as a follow-up.


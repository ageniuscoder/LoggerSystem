<div align="center">

<h1>mlog</h1>

<p>A fast, structured, asynchronous logging library for Go.</p>

<p>
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version"/>
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat" alt="License"/>
  <img src="https://img.shields.io/badge/tests-passing-4caf50?style=flat" alt="Tests"/>
  <img src="https://img.shields.io/badge/race--safe-yes-4caf50?style=flat" alt="Race Safe"/>
</p>

<p>
Non-blocking · Batch I/O · Zero-alloc formatting · JSON &amp; text output · Rotating files
</p>

</div>

---

## Why mlog

Most loggers block your goroutines on every write. mlog writes to a buffered channel and returns immediately — a single background worker drains the channel in batches and flushes to disk. Your hot path pays one channel send, not a syscall.

```
goroutine → channel → batchWorker → handler chain → appender → formatter
              ↑                                                      ↓
          non-blocking                                        disk / stderr
```

- **Async by default** — log calls never block, never slow down your application
- **Batch I/O** — multiple messages written in one syscall, configurable batch size
- **Zero-alloc formatting** — `sync.Pool` byte buffers reused across every log call
- **Structured fields** — typed key/value pairs, not string concatenation
- **JSON and text** — machine-parseable JSON for log aggregators, human-readable text for development
- **Rotating files** — size and age based rotation via lumberjack, gzip compression support
- **Race-safe** — tested with `-race` across all packages
- **Dropped message tracking** — `DroppedCount()` tells you exactly when back-pressure occurs

---

## Installation

```bash
go get github.com/ageniuscoder/mlog
```

---

## Quick start

```go
package main

import "github.com/ageniuscoder/mlog"

func main() {
    log, stop := mlog.Default()
    defer stop()

    log.Info("server started", mlog.M("port", 8080), mlog.M("env", "prod"))
    log.Error("db failed",     mlog.M("host", "db01"), mlog.M("err", err))
}
```

> **Always defer `stop()`.** mlog is async — the last batch of messages sits in the channel until the worker flushes it. Without `stop()`, those messages are silently dropped when the process exits.

---

## Constructors

Pick the one that fits your situation. All three return the same `*Logger`.

### `Default()` — zero config

```go
log, stop := mlog.Default()
defer stop()
```

| Setting | Value         |
| ------- | ------------- |
| Output  | stderr        |
| Level   | info          |
| Format  | text          |
| Buffer  | 4096 messages |

### `New()` — code config

No config file needed. Options are validated at startup — `New` panics on bad input so misconfiguration blows up immediately, not silently at runtime.

```go
log, stop := mlog.New(
    mlog.WithLevel("info"),
    mlog.WithRotatingFile("./logs/app.log", 100, 30, 5),
    mlog.WithJSON(),
)
defer stop()
```

### `FromFile()` — JSON config

For ops teams who need per-environment configuration without recompiling.

```go
log, stop, err := mlog.FromFile("./logger.json")
if err != nil {
    panic(err)
}
defer stop()
```

---

## Logging methods

```go
log.Debug("cache miss",    mlog.M("key", "user:42"), mlog.M("hit", false))
log.Info("request done",   mlog.M("method", "GET"),  mlog.M("status", 200), mlog.M("ms", 12.4))
log.Warning("disk low",    mlog.M("free_gb", 2),     mlog.M("path", "/var"))
log.Error("query failed",  mlog.M("table", "orders"),mlog.M("err", err))
log.Fatal("config missing",mlog.M("path", "./config.json"))
```

All methods are safe to call concurrently from multiple goroutines.

---

## Structured fields — `M()`

`M` detects the value type automatically. One function covers every common case.

```go
mlog.M("port",    8080)        // int
mlog.M("host",    "localhost") // string
mlog.M("ok",      true)        // bool
mlog.M("latency", 1.24)        // float64
mlog.M("err",     err)         // error — nil-safe, captures err.Error()
mlog.M("req",     myStruct)    // any   — formatted with %+v
```

Typed constructors are also available when you want to be explicit:

```go
mlog.M("id",  logmsg.Int64Field("id", userID))   // int64
mlog.M("val", logmsg.Float64Field("v", 3.14))    // float64
```

---

## Output formats

### Text

Human-readable. Good for development terminals and simple log tailing.

```
[info]-    2026-03-15 12:00:00 main.go:42: server started | port=8080 | env="prod"
[warning]- 2026-03-15 12:00:01 disk.go:18: disk low | free_gb=2 | path="/var"
[error]-   2026-03-15 12:00:02 db.go:87:   query failed | table="orders" | err="connection refused"
```

### JSON

Machine-parseable. Good for log aggregators (Datadog, Loki, Elasticsearch, CloudWatch).

```json
{"timestamp":"2026-03-15 12:00:00","level":"info","caller":"main.go:42","msg":"server started","port":8080,"env":"prod"}
{"timestamp":"2026-03-15 12:00:01","level":"warning","caller":"disk.go:18","msg":"disk low","free_gb":2,"path":"/var"}
{"timestamp":"2026-03-15 12:00:02","level":"error","caller":"db.go:87","msg":"query failed","table":"orders","err":"connection refused"}
```

---

## Common patterns

### Development — console, all levels, text

```go
log, stop := mlog.New(
    mlog.WithLevel("debug"),
    mlog.WithConsole("text"),
)
defer stop()
```

### Production — rotating file, JSON

```go
log, stop := mlog.New(
    mlog.WithLevel("info"),
    mlog.WithRotatingFile("./logs/app.log", 100, 30, 5),
    mlog.WithJSON(),
)
defer stop()
```

### Split output — text console + JSON file

```go
log, stop := mlog.New(
    mlog.WithLevel("debug"),
    mlog.WithConsole("text"),                          // human-readable to stderr
    mlog.WithRotatingFile("./logs/app.log", 100, 30, 5),
    mlog.WithJSON(),                                   // JSON to file only
)
defer stop()
```

### Wrapping mlog in your own logger

If you add one wrapper function around mlog, the caller reported in log output will point to your wrapper, not the actual call site. Fix this with `WithSkip`:

```go
type AppLogger struct {
    inner *mlog.Logger
    stop  func()
}

func New() *AppLogger {
    log, stop := mlog.New(
        mlog.WithSkip(5), // default is 4; +1 for each wrapper layer
        mlog.WithRotatingFile("./logs/app.log", 100, 30, 5),
    )
    return &AppLogger{inner: log, stop: stop}
}

func (l *AppLogger) Info(msg string, fields ...mlog.Field) {
    l.inner.Info(msg, fields...)
}

func (l *AppLogger) Close() { l.stop() }
```

### Monitoring back-pressure

```go
// Check periodically — e.g. in a metrics goroutine
if n := log.DroppedCount(); n > 0 {
    metrics.Gauge("log.dropped", float64(n))
}
```

A non-zero count means the buffer fills faster than the worker can flush. Solutions: increase `WithBuffer`, reduce log volume, or investigate slow disk I/O.

---

## Options reference

| Option                                                      | Default  | Description                                                                                                                                                    |
| ----------------------------------------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `WithLevel(level)`                                          | `"info"` | Minimum level. One of `debug` `info` `warning` `error` `fatal`. Messages below this are discarded before entering the buffer — cost is one integer comparison. |
| `WithConsole(formatter)`                                    | —        | Write to stderr. `"text"` or `"json"`.                                                                                                                         |
| `WithFile(path)`                                            | —        | Write to a plain file. Created if absent, appended to if present. Parent directory created automatically.                                                      |
| `WithRotatingFile(path, maxSizeMB, maxAgeDays, maxBackups)` | —        | Rotating file. Rotates at `maxSizeMB` MB. Deletes files older than `maxAgeDays` days. Keeps at most `maxBackups` old files.                                    |
| `WithJSON()`                                                | —        | Switch the last-added appender to JSON format. Chain immediately after `WithConsole`, `WithFile`, or `WithRotatingFile`.                                       |
| `WithBuffer(n)`                                             | `4096`   | Async channel capacity. Increase for high-throughput services. Dropped messages increment `DroppedCount()`.                                                    |
| `WithBatchSize(n)`                                          | `256`    | Flush after this many messages. Lower = lower latency. Higher = better disk throughput.                                                                        |
| `WithFlushInterval(ms)`                                     | `100`    | Force a flush after this many milliseconds. Bounds the maximum age of a buffered message.                                                                      |
| `WithSkip(n)`                                               | `4`      | Stack frames to skip for caller identification. Add 1 for each wrapper layer.                                                                                  |

---

## JSON config file

Full reference for `FromFile`. Fields with defaults can be omitted.

```json
{
  "min_level": "info",
  "min_skip": 4,
  "buffer": 4096,
  "batch_size": 256,
  "flush_interval": 100,
  "levels": [
    {
      "level": "info",
      "appenders": [
        {
          "name": "stdout",
          "type": "console",
          "formatter": { "type": "text" }
        }
      ]
    },
    {
      "level": "error",
      "appenders": [
        {
          "name": "error-file",
          "type": "rotating_file",
          "formatter": { "type": "json" },
          "path": "./logs/error.log",
          "max_size": 100,
          "max_age": 30,
          "max_backups": 5,
          "local_time": true,
          "compress": false
        }
      ]
    }
  ]
}
```

<details>
<summary>Config field reference</summary>

**Top-level**

| Field            | Type   | Constraints                              | Default   | Description             |
| ---------------- | ------ | ---------------------------------------- | --------- | ----------------------- |
| `min_level`      | string | `debug` `info` `warning` `error` `fatal` | `"debug"` | Minimum log level       |
| `min_skip`       | int    | 1–10                                     | `4`       | Caller stack skip depth |
| `buffer`         | int    | 0–100 000                                | `4096`    | Async channel capacity  |
| `batch_size`     | int    | 1–10 000                                 | `256`     | Messages per flush      |
| `flush_interval` | int    | 10–60 000                                | `100`     | Flush interval (ms)     |

**Level object**

| Field       | Type   | Required | Description                              |
| ----------- | ------ | -------- | ---------------------------------------- |
| `level`     | string | yes      | `debug` `info` `warning` `error` `fatal` |
| `appenders` | array  | yes      | One or more appender objects             |

**Appender object**

| Field         | Type   | Required         | Description                            |
| ------------- | ------ | ---------------- | -------------------------------------- |
| `name`        | string | yes              | Unique name                            |
| `type`        | string | yes              | `console` `file` `rotating_file`       |
| `formatter`   | object | yes              | `{"type":"text"}` or `{"type":"json"}` |
| `path`        | string | if file/rotating | Output file path                       |
| `max_size`    | int    | if rotating      | MB before rotation                     |
| `max_age`     | int    | no               | Days to keep old files (0 = forever)   |
| `max_backups` | int    | no               | Max old files to keep                  |
| `local_time`  | bool   | no               | Local time in rotation filenames       |
| `compress`    | bool   | no               | Gzip rotated files                     |

</details>

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  your goroutines                    │
│   log.Info(...)   log.Error(...)   log.Debug(...)   │
└──────────────┬──────────────────────────────────────┘
               │  non-blocking channel send
               ▼
┌─────────────────────────┐
│   channel  (buffer=N)   │  drops + counts if full
└──────────┬──────────────┘
           │  single batchWorker goroutine — guarantees FIFO
           ▼
┌──────────────────────────────────────────────────────┐
│            handler chain (Chain of Responsibility)   │
│   DebugHandler → InfoHandler → WarningHandler → ...  │
└──────────────────────────┬───────────────────────────┘
                           │
           ┌───────────────┼──────────────────┐
           ▼               ▼                  ▼
    ConsoleAppender   FileAppender   RotatingFileAppender
           │               │                  │
           ▼               ▼                  ▼
    TextFormatter     JsonFormatter     JsonFormatter
      (sync.Pool)      (sync.Pool)       (sync.Pool)
```

Each handler in the chain owns exactly one log level. A message at `ERROR` travels through `DebugHandler`, `InfoHandler`, `WarningHandler` (all forward it) and is consumed by `ErrorHandler`. This means you can attach different appenders to different levels — send `debug` to console and `error` to a separate file — with zero coupling between them.

---

## Dependencies

| Package                                                                                | Version  | Purpose                |
| -------------------------------------------------------------------------------------- | -------- | ---------------------- |
| [`gopkg.in/natefinch/lumberjack.v2`](https://github.com/natefinch/lumberjack)          | v2.2.1   | Rotating file I/O      |
| [`github.com/go-playground/validator/v10`](https://github.com/go-playground/validator) | v10.30.1 | JSON config validation |

---

## License

[MIT](LICENSE)

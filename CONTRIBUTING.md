# Contributing to mlog

Thank you for taking the time to contribute. mlog is a small, focused library — contributions that keep it fast, simple, and well-tested are always welcome.

---

## Table of contents

- [Getting started](#getting-started)
- [Project structure](#project-structure)
- [Development workflow](#development-workflow)
- [Running tests](#running-tests)
- [What to contribute](#what-to-contribute)
- [What not to contribute](#what-not-to-contribute)
- [Code style](#code-style)
- [Commit messages](#commit-messages)
- [Opening a pull request](#opening-a-pull-request)
- [Reporting bugs](#reporting-bugs)
- [Suggesting features](#suggesting-features)

---

## Getting started

**Prerequisites**

- Go 1.21 or later
- Git

**Fork and clone**

```bash
# Fork the repo on GitHub first, then:
git clone https://github.com/YOUR_USERNAME/mlog.git
cd mlog
go mod download
```

**Verify everything works before you change anything**

```bash
go test ./... -race -timeout 30s
```

All packages should pass. If anything fails on a clean clone, open an issue before continuing.

---

## Project structure

```
mlog/
├── mlog.go              # Public API — Logger, M(), Default(), New(), FromFile()
├── options.go           # Functional options — WithLevel, WithFile, WithJSON, etc.
├── mlog_test.go         # End-to-end tests against the public API
│
└── internal/
    ├── logmsg/          # Core types — LogMsg, Field, LogLevel, sync.Pool
    ├── formatter/       # TextFormatter and JsonFormatter (zero-alloc, sync.Pool)
    ├── appender/        # ConsoleAppender, FileAppender, RotatingFileAppender
    ├── handler/         # Chain-of-Responsibility handlers per log level
    ├── logger/          # Async Logger — channel, batchWorker goroutine, Shutdown
    └── config/          # Build, BuildFromOptions, Load, JSON parser, validation
```

**The dependency direction is strict and must not be reversed:**

```
mlog → config → logger → handler → appender → formatter → logmsg
```

No package may import a package above it in this chain. `logmsg` is the leaf — it imports nothing from this module.

---

## Development workflow

```bash
# Create a branch for your change
git checkout -b your-feature-or-fix

# Make changes, then verify
go build ./...
go vet ./...
go test ./... -race -timeout 30s

# If you added a new file or dependency
go mod tidy
```

Do not commit directly to `main`. All changes go through a pull request.

---

## Running tests

```bash
# Run all tests with the race detector — mandatory before any PR
go test ./... -race -timeout 30s

# Run a specific package
go test ./internal/formatter/... -race -v

# Run a single test by name
go test ./internal/logger/... -race -run TestShutdown_DrainsPendingMessages -v

# Run with verbose output to see all test names
go test ./... -race -v
```

**The race detector is not optional.** mlog is a concurrent library — a change that passes tests without `-race` but fails with it is a broken change.

---

## What to contribute

These are areas where contributions are genuinely useful:

**Bug fixes**
Any correctness issue, race condition, or behaviour that diverges from what the documentation says. Always include a test that fails before your fix and passes after.

**New appender types**
For example: a `SyslogAppender`, a `MultiAppender` that fans out to a list of appenders, or a `BufferedAppender` that wraps another appender. A new appender must implement the `LogAppender` interface:

```go
type LogAppender interface {
    AppendMsg(msg *logmsg.LogMsg) error
    AppendBatch(msgs []*logmsg.LogMsg) error
}
```

**New formatter types**
For example: a `LogfmtFormatter` for [logfmt](https://brandur.org/logfmt) output. A new formatter must implement:

```go
type LogFormatter interface {
    Format(msg *logmsg.LogMsg) string
}
```

**New config file formats**
mlog uses a self-registering parser system. Adding YAML support is as simple as implementing `Parser` and calling `Register("yaml", &YamlParser{})` in an `init()` function. See `internal/config/jsonparser.go` for the pattern.

**Performance improvements**
If you have a benchmark showing a meaningful improvement, that is welcome. Always include the benchmark output in the PR description. Benchmarks live alongside test files:

```go
func BenchmarkTextFormatter_Format(b *testing.B) {
    f := NewTextFormatter()
    msg := &logmsg.LogMsg{ /* ... */ }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = f.Format(msg)
    }
}
```

Run with:

```bash
go test ./internal/formatter/... -bench=. -benchmem
```

**Documentation improvements**
Godoc comments, README corrections, or clearer examples in `mlog.go` and `options.go` are always welcome.

---

## What not to contribute

To keep mlog focused, please **do not** open PRs for:

- **Global/singleton logger** — `mlog.Info(...)` without a logger instance. This is an explicit non-goal; it makes testing harder and creates hidden global state.
- **`log/slog` handler adapter** — out of scope for this library.
- **Sampling or rate-limiting** — adds complexity that belongs in the caller.
- **Log rotation beyond lumberjack** — the current rotating file appender covers the common case. Cloud-native rotation (e.g. Docker log drivers) belongs outside the library.
- **Breaking changes to the public API** — `mlog.go` and `options.go` are the stable surface. Changes that break existing callers require a v2 module path and a strong justification.

If you are unsure whether something is in scope, open a GitHub issue and ask before writing code.

---

## Code style

Follow standard Go conventions. A few specifics that matter for this codebase:

**No allocations on the hot path.**
The formatters use `sync.Pool` byte-buffer reuse for a reason. Any change to `TextFormatter.Format` or `JsonFormatter.Format` must not introduce heap allocations per call. Verify with:

```bash
go test ./internal/formatter/... -bench=. -benchmem
```

`allocs/op` must stay at 0 for the formatter benchmarks.

**Keep the internal packages internal.**
Nothing in `internal/` is part of the public API. Do not add exported symbols to internal packages unless they are needed by another internal package.

**Lock discipline.**
`BaseHandler` uses `sync.RWMutex`. Read the existing lock patterns before touching `handle.go` — copying the slice under `RLock` before doing I/O is intentional and must be preserved.

**Error handling.**
Appender errors are logged to `os.Stderr` by the handler — they do not propagate to the caller. Do not change this contract; callers of `log.Info(...)` must never receive an error return.

**`gofmt` and `go vet` must pass.**

```bash
gofmt -l .        # should print nothing
go vet ./...      # should print nothing
```

---

## Commit messages

Use the conventional format:

```
<type>: <short summary in present tense>

<optional body — explain why, not what>
```

**Types:**

| Type       | When to use                                         |
| ---------- | --------------------------------------------------- |
| `feat`     | New feature or behaviour                            |
| `fix`      | Bug fix                                             |
| `perf`     | Performance improvement                             |
| `test`     | Adding or fixing tests                              |
| `docs`     | Documentation only                                  |
| `refactor` | Code change with no behaviour change                |
| `chore`    | Maintenance — dependency updates, go mod tidy, etc. |

**Examples:**

```
feat: add SyslogAppender for Unix syslog output

fix: zero Level and Timestamp fields in PutMsgPool

perf: avoid string allocation in JsonFormatter.Format

docs: add contributing guide
```

Keep the summary under 72 characters. Use the body to explain _why_ if the change is non-obvious.

---

## Opening a pull request

1. Make sure `go test ./... -race -timeout 30s` passes locally.
2. Make sure `gofmt -l .` and `go vet ./...` produce no output.
3. Write or update tests for your change. PRs that reduce test coverage will not be merged.
4. Fill out the PR description with:
   - **What** the change does
   - **Why** it is needed
   - **How** you tested it (include benchmark output for performance changes)
5. Keep PRs focused — one logical change per PR. A PR that fixes a bug and adds a feature will be asked to split.

---

## Reporting bugs

Open a GitHub issue with:

- **Go version** (`go version`)
- **mlog version** (the tag or commit)
- **Operating system and architecture**
- **Minimal reproducible example** — the smallest code that triggers the bug
- **Expected behaviour** vs **actual behaviour**
- If it is a concurrency bug, include the race detector output

---

## Suggesting features

Open a GitHub issue with the label `enhancement`. Describe:

- The use case — what problem does this solve?
- A rough sketch of the API you have in mind
- Whether you are willing to implement it yourself

Features that fit the design philosophy (fast, non-blocking, zero-alloc hot path, clean public API) and come with a willing implementor are much more likely to be accepted.

---

## Questions

If something in the codebase is unclear, open a GitHub Discussion rather than an issue. Issues are for bugs and concrete feature requests.

// Package mlog is a fast, structured, asynchronous logging library.
//
// Three constructors — pick the one that fits your situation:
//
//	// 1. Zero config — console, info level, text format.
//	log, stop := mlog.Default()
//	defer stop()
//
//	// 2. Code config — no JSON file needed.
//	log, stop := mlog.New(
//	    mlog.WithLevel("info"),
//	    mlog.WithRotatingFile("./logs/app.log", 100, 14, 5),
//	)
//	defer stop()
//
//	// 3. File config — JSON file, good for ops teams.
//	log, stop, err := mlog.FromFile("./logger.json")
//	if err != nil { panic(err) }
//	defer stop()
//
// All three return the same *Logger:
//
//	log.Info("server started", mlog.M("port", 8080))
//	log.Error("db failed",     mlog.M("host", "db01"), mlog.M("err", err))
//
// Always call stop() — it blocks until the async batchWorker has flushed
// every queued message. Skipping it silently drops the last batch on exit.
package mlog

import (
	"github.com/ageniuscoder/mlog/internal/config"
	"github.com/ageniuscoder/mlog/internal/logmsg"
)

// Logger is the only type your application needs to hold.
// Obtain one from Default(), New(), or FromFile().
// All methods are safe to call concurrently from multiple goroutines.
// The zero value is not usable — always use a constructor.
type Logger struct {
	// core is unexported — callers never reach internal types directly.
	// This lets internals change in future versions without
	// breaking any user code.
	core *config.CoreLogger
}


// Field is a single structured key=value pair attached to a log entry.
// Never construct this directly — always use M().
type Field = logmsg.Field

// M creates a structured log field. The value type is detected automatically.
//
//	mlog.M("port",    8080)        // int
//	mlog.M("host",    "localhost") // string
//	mlog.M("ok",      true)        // bool
//	mlog.M("latency", 1.24)        // float64
//	mlog.M("err",     err)         // error  — nil-safe
//	mlog.M("obj",     myStruct)    // any    — formatted with %+v
//  mlog.M("key",     any)         // any    — formatted with +v
func M(key string, val any) Field {
	return logmsg.M(key, val)
}


// Default returns a ready-to-use logger with no configuration required.
//
// Defaults:
//   - Output:  stderr (console)
//   - Level:   info
//   - Format:  text
//   - Buffer:  4096 messages
//
// Always defer the returned stop function. The logger is async — messages
// sit in a channel until the batchWorker flushes them. Without stop(),
// the process can exit before the worker runs, silently dropping the last
// batch of messages.
//
//	log, stop := mlog.Default()
//	defer stop()
//	log.Info("hello", mlog.M("version", "1.0.0"))
//
// stop() for Default() only calls Shutdown() — there are no files to close.
// Console writes to stderr directly; stderr needs no explicit closing.
// For file output use New() or FromFile() instead.
func Default() (*Logger, func()) {
	core, _, _ := config.BuildFromOptions(defaultOptions())
	// drain the channel before the process exits.
	stop := func() {
		core.Shutdown()
	}
	return &Logger{core: core}, stop
}



// New builds a Logger from functional options.
// Returns the logger and a stop function.
//
// Always defer stop() — it flushes buffered messages and closes open files:
//
//	log, stop := mlog.New(
//	    mlog.WithLevel("info"),
//	    mlog.WithConsole("text"),
//	    mlog.WithRotatingFile("./logs/app.log", 100, 14, 5),
//	)
//	defer stop()
//
// Calling New() with no options behaves like Default() — console output
// at info level. For purely console use, Default() is the cleaner choice.
//
// New() panics on invalid options (e.g. WithRotatingFile with an empty path).
// should blow up at startup, not silently fail at runtime.
func New(opts ...Option) (*Logger, func()) {
	o := defaultOptions()
	for _, opt := range opts {    //opt are actually func(*config.Options)
		opt(&o)                    //here we pass o which is config.Options, and opt modfies it
	}

	core, closers, err := config.BuildFromOptions(o)   //here modified o after all opt applied
	if err != nil {
		panic("mlog: New() received invalid options: " + err.Error())
	}

	return &Logger{core: core}, newStopFunc(core, closers)
}


// FromFile builds a Logger from a JSON config file.
// Returns the logger, a stop function, and an error.
//
// (different logger.json per environment).
//
//	log, stop, err := mlog.FromFile("./logger.json")
//	if err != nil {
//	    panic(err)
//	}
//	defer stop()
//
// Returns an error if the file cannot be read, has invalid JSON, or fails
// validation. stop is nil when err is non-nil — never call it in that case.
func FromFile(path string) (*Logger, func(), error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, nil, err
	}

	core, closers, err := config.Build(cfg)
	if err != nil {
		return nil, nil, err
	}

	return &Logger{core: core}, newStopFunc(core, closers), nil
}


// Debug logs at debug level.
// No-op when the logger's minimum level is above debug.
//
//	log.Debug("cache lookup", mlog.M("key", "user:42"), mlog.M("hit", false))
func (l *Logger) Debug(msg string, fields ...Field) {
	l.core.Debug(msg, fields)
}

// Info logs at info level.
//
//	log.Info("server started", mlog.M("port", 8080), mlog.M("env", "prod"))
func (l *Logger) Info(msg string, fields ...Field) {
	l.core.Info(msg, fields)
}

// Warning logs at warning level.
//
//	log.Warning("disk space low", mlog.M("free_gb", 2), mlog.M("path", "/var"))
func (l *Logger) Warning(msg string, fields ...Field) {
	l.core.Warning(msg, fields)
}

// Error logs a message at the error level.
//
// Use this for errors that are relevant to the operation of the application 
// but do not require immediate termination.
//
//  log.Error("db connection failed", mlog.M("host", "db01"), mlog.M("err", err))
func (l *Logger) Error(msg string, fields ...Field) {
    l.core.Error(msg, fields)
}

// Fatal logs a message at the fatal level.
//
// Fatal represents the highest log level, typically used for unrecoverable 
// errors that require the application to exit or halt.
//
//  log.Fatal("failed to load configuration", mlog.M("path", "./config.json"), mlog.M("err", err))
func (l *Logger) Fatal(msg string, fields ...Field) {
    l.core.Fatal(msg, fields)
}

// DroppedCount returns the total number of log messages dropped since
// the logger was created.
//
// A message is dropped when the internal buffer channel is full at the
// moment log() is called. The caller is never blocked — the message is
// simply discarded and counted.
//
// Monitor this in production to detect back-pressure:
//
//	if n := log.DroppedCount(); n > 0 {
//	    fmt.Printf("WARNING: %d log messages were dropped\n", n)
//	}
//
// Non-zero means: increase WithBuffer(), reduce log volume, or investigate
// slow disk I/O on your file appenders.
func (l *Logger) DroppedCount() int64 {
	return l.core.GetDroppedLogsCnt()
}

//Private function
// newStopFunc combines Shutdown() and file closing into one zero-argument
// closure. Used by New() and FromFile() — both may have file closers.
//
// Order is load-bearing:
//  1. Shutdown() — signals done, then blocks on wg.Wait() until batchWorker
//     has fully drained the channel and written the very last batch.
//  2. closers   — safe only after Shutdown() returns, because no more
//     writes can happen once batchWorker has exited.
//
// Reversing the order would close files while batchWorker is still writing
// to them — corrupting the last batch.
func newStopFunc(core *config.CoreLogger, closers []func() error) func() {
	return func() {
		core.Shutdown()
		for _, c := range closers {
			c()
		}
	}
}
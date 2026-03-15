package mlog

import "github.com/ageniuscoder/mlog/internal/config"

// Option is a function that configures a Logger.
// Pass one or more to New() — each WithXxx function returns one Option.
type Option func(*config.Options)

// defaultOptions is the baseline New() starts from before applying any
// user-supplied options.
//
// A console/text appender is seeded here so that New() with no appender
// options still produces visible output — same behaviour as Default().
// The moment the user provides any explicit appender via WithConsole,
// WithFile, or WithRotatingFile, this seed is replaced by replaceSeededConsole().
func defaultOptions() config.Options {
	return config.Options{
		MinLevel:      "info",
		Buffer:        4096,
		BatchSize:     256,
		FlushInterval: 100,
		Appenders: []config.AppenderOption{
			{Type: "console", Formatter: "text"},
		},
	}
}

// WithLevel sets the minimum log level.
// Messages below this level are discarded before entering the async buffer,
// so they cost almost nothing — just one integer comparison.
//
// Valid values: "debug" | "info" | "warning" | "error"
// Default:      "info"
//
//	mlog.New(mlog.WithLevel("debug"))    // see everything
//	mlog.New(mlog.WithLevel("warning"))  // warnings and errors only
//	mlog.New(mlog.WithLevel("error"))    // errors only
func WithLevel(level string) Option {
	return func(o *config.Options) {
		o.MinLevel = level
	}
}


// WithConsole adds a console appender that writes to stderr.
//
// The formatter argument controls the output style:
//   - "text" → [info]- 2026-03-15 12:00:00 main.go:42: message | key=value
//   - "json" → {"timestamp":"...","level":"info","caller":"main.go:42","msg":"...","key":value}
//
//	mlog.New(mlog.WithConsole("text"))  // human-readable, good for development
//	mlog.New(mlog.WithConsole("json"))  // machine-parseable, good for log aggregators
//
// The first call to WithConsole replaces the seeded default console appender.
// Calling it a second time adds a second console appender (rarely useful).
func WithConsole(formatter string) Option {
	return func(o *config.Options) {
		o.Appenders = replaceSeededConsole(o.Appenders,
			config.AppenderOption{Type: "console", Formatter: formatter},
		)
	}
}

// WithFile adds a plain (non-rotating) file appender.
// The file is created if it does not exist; logs are appended if it does.
// The parent directory is created automatically.
//
//	mlog.New(mlog.WithFile("./app.log", "text"))
//	mlog.New(mlog.WithFile("./app.log", "json"))
//
// For long-running services, prefer WithRotatingFile so the file does
// not grow unbounded.
func WithFile(path, formatter string) Option {
	return func(o *config.Options) {
		o.Appenders = replaceSeededConsole(o.Appenders,
			config.AppenderOption{Type: "file", Path: path, Formatter: formatter},
		)
	}
}

// WithRotatingFile adds a size-and-age rotating file appender.
//
//   - maxSizeMB  — rotate when the file exceeds this size in megabytes
//   - maxAgeDays — delete rotated files older than this many days (0 = keep forever)
//
// The parent directory is created automatically.
// Format defaults to "text". Chain WithJSON() immediately after to get JSON:
//
//	// text rotation
//	mlog.New(
//	    mlog.WithRotatingFile("./logs/app.log", 100, 14),
//	)
//
//	// json rotation
//	mlog.New(
//	    mlog.WithRotatingFile("./logs/app.log", 100, 14),
//	    mlog.WithJSON(),
//	)
//
//	// console AND rotating file together
//	mlog.New(
//	    mlog.WithConsole("text"),
//	    mlog.WithRotatingFile("./logs/app.log", 100, 14),
//	)
func WithRotatingFile(path string, maxSizeMB, maxAgeDays,maxBackups int) Option {
	return func(o *config.Options) {
		o.Appenders = replaceSeededConsole(o.Appenders,
			config.AppenderOption{
				Type:       "rotating_file",
				Path:       path,
				Formatter:  "text", // WithJSON() can override this
				MaxSize:    maxSizeMB,
				MaxAge:     maxAgeDays,
				MaxBackups: maxBackups,
				LocalTime:  true,
			},
		)
	}
}

// WithJSON switches the most recently added appender to JSON output format.
//
// Chain it immediately after WithConsole, WithFile, or WithRotatingFile.
// It modifies the last element in the appenders slice — not a standalone
// appender — so it must always follow an appender option.
// It has no effect if the appenders slice is empty.
//
//	// rotating file with JSON output
//	mlog.New(
//	    mlog.WithRotatingFile("./logs/app.log", 100, 14),
//	    mlog.WithJSON(),
//	)
//
//	// two appenders: text console + json file
//	mlog.New(
//	    mlog.WithConsole("text"),
//	    mlog.WithRotatingFile("./logs/app.log", 100, 14),
//	    mlog.WithJSON(),  // switches only ./logs/app.log to JSON
//	)
func WithJSON() Option {
	return func(o *config.Options) {
		n := len(o.Appenders)
		if n == 0 {
			return
		}
		o.Appenders[n-1].Formatter = "json"
	}
}

// WithBuffer sets the capacity of the internal async channel.
//
// This is the maximum number of log messages that can queue in memory
// before the logger starts dropping them under load. Dropped messages
// are counted by Logger.DroppedCount() — never silently ignored.
//
// Default: 4096
//
//	mlog.New(mlog.WithBuffer(16384))  // high-throughput service
//	mlog.New(mlog.WithBuffer(512))    // low-memory embedded system
func WithBuffer(n int) Option {
	return func(o *config.Options) {
		o.Buffer = n
	}
}

// WithBatchSize sets how many messages are written to disk in a single syscall.
//
// The batchWorker flushes when either:
//   - the batch reaches BatchSize messages, OR
//   - FlushInterval milliseconds have elapsed (whichever comes first)
//
// Default: 256
//
// Higher values improve disk throughput at the cost of increased latency
// before messages appear in the file. Lower values reduce latency.
//
//	mlog.New(mlog.WithBatchSize(1024))  // maximise throughput
//	mlog.New(mlog.WithBatchSize(1))     // flush every message immediately
func WithBatchSize(n int) Option {
	return func(o *config.Options) {
		o.BatchSize = n
	}
}

// WithFlushInterval sets the maximum time in milliseconds between forced flushes.
//
// Even if the batch has not yet reached BatchSize, the worker flushes after
// this interval elapses. This bounds the maximum age of a buffered message.
//
// Default: 100ms
//
//	mlog.New(mlog.WithFlushInterval(50))    // flush at most every 50ms
//	mlog.New(mlog.WithFlushInterval(1000))  // flush at most every 1 second
func WithFlushInterval(ms int) Option {
	return func(o *config.Options) {
		o.FlushInterval = ms
	}
}


// replaceSeededConsole enforces the default-appender contract:
//
// defaultOptions() seeds one console/text appender so New() with no appender
// options still produces output. But the moment the user provides their first
// explicit appender, that seed must disappear — otherwise programs get
// unexpected extra console output even when they configured file-only logging.
//
// Rule:
//   - If the current slice is exactly the seed (one entry, console, text),
//     replace it entirely with next.
//   - Otherwise the user has already made explicit choices — append next.
//
// This makes all combinations work correctly:
//
//	New()                                     → console/text (seed kept)
//	New(WithRotatingFile(...))                → file only    (seed replaced)
//	New(WithConsole("json"))                  → json console (seed replaced)
//	New(WithConsole("text"), WithFile(...))   → both         (seed replaced, file appended)
func replaceSeededConsole(current []config.AppenderOption, next config.AppenderOption) []config.AppenderOption {
	if len(current) == 1 &&
		current[0].Type == "console" &&
		current[0].Formatter == "text" {
		return []config.AppenderOption{next}
	}
	return append(current, next)
}
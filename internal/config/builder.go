package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ageniuscoder/mlog/internal/appender"
	"github.com/ageniuscoder/mlog/internal/formatter"
	logs "github.com/ageniuscoder/mlog/internal/logger"
	"github.com/ageniuscoder/mlog/internal/logmsg"
)

// CoreLogger is a type alias that lets the root mlog package hold a *Logger
// without importing internal/logger directly.
// A type alias (=) means it is the exact same type — no wrapping, no overhead.
type CoreLogger = logs.Logger

func Build(cfg *LoggerConfig) (*logs.Logger, []func() error, error) {
	var closers []func() error
	level, ok := logmsg.ParseLevel(cfg.MinLevel)
	if !ok {
		return nil, closers, fmt.Errorf("Config: not valid log level: %q", cfg.MinLevel)
	}
	system := logs.NewLogger(cfg.Buffer, level, cfg.BatchSize,cfg.MinSkip,time.Duration(cfg.FlushInterval)*time.Millisecond)

	for _, lv := range cfg.Levels {
		for _, ac := range lv.Appenders {

			f, err := buildFormatter(ac.Formatter)
			if err != nil {
				closeAll(system,closers)
				return nil, nil, err
			}

			a, closer, err := buildAppender(ac, f)
			if err != nil {
				closeAll(system,closers)
				return nil, nil, err
			}
			if closer != nil {
				closers = append(closers, closer)
			}

			system.AddAppender(lv.Level, a)
		}
	}
	return system, closers, nil
}

func buildFormatter(fc formatterConfig) (formatter.LogFormatter, error) {
	switch fc.Type {
	case "text":
		return formatter.NewTextFormatter(), nil
	case "json":
		return formatter.NewJsonFormatter(), nil
	default:
		return nil, fmt.Errorf("builder: unknown formatter type %q", fc.Type)
	}
}

func buildAppender(ac appenderConfig, f formatter.LogFormatter) (appender.LogAppender, func() error, error) {
	switch ac.Type {
	case "console":
		return appender.NewConsoleAppender(f), nil, nil
	case "file":
		fa, err := appender.NewFileAppender(ac.Path, f)
		if err != nil {
			return nil, nil, fmt.Errorf("builder: cannot open log file %q: %w", ac.Path, err)
		}
		return fa, fa.CloseFile, nil
	case "rotating_file":
		os.MkdirAll(filepath.Dir(ac.Path), 0755)
		fa := appender.NewRotatingFileAppender(ac.Path, ac.MaxSize, ac.MaxAge, ac.MaxBackups, ac.LocalTime, ac.Compress, f)
		return fa, fa.CloseFile, nil
	default:
		return nil, nil, fmt.Errorf("builder: unknown appender type %q", ac.Type)
	}
}

func closeAll(sys *CoreLogger,closers []func() error) {
	sys.Shutdown()
    for _, c := range closers {
        c()
    }
}


// buildFormatterByName builds a formatter from a plain string name.
// Used by BuildFromOptions where there is no formatterConfig struct.
func buildFormatterByName(name string) formatter.LogFormatter {
    if name == "json" {
        return formatter.NewJsonFormatter()
    }
    return formatter.NewTextFormatter() // default — never fails
}

// BuildFromOptions wires up a Logger from the programmatic Options struct.
// Called by mlog.New() and mlog.Default() — no config file needed.
//
// Key difference from Build(): every appender is attached to ALL level handlers
// using AddAppender(""). The Logger's own minLevel gate prevents messages below
// the threshold from ever reaching any handler, so there are no duplicate writes.

func BuildFromOptions(o Options) (*logs.Logger, []func() error, error) {
    // Apply defaults for any zero values so callers don't have to set everything.
    o = applyOptionDefaults(o)

    level, ok := logmsg.ParseLevel(o.MinLevel)
    if !ok {
        return nil, nil, fmt.Errorf("mlog: invalid level %q (want: debug|info|warning|error)", o.MinLevel)
    }

    sys := logs.NewLogger(
        o.Buffer, level,
        o.BatchSize,
		o.MinSkip,
        time.Duration(o.FlushInterval)*time.Millisecond,
    )

    var closers []func() error

    for _, ao := range o.Appenders {
        if err := validateAppenderOption(ao); err != nil {
            closeAll(sys,closers)
            return nil, nil, err
        }
		f := buildFormatterByName(ao.Formatter)

        a, closer, err := buildAppenderFromOption(ao, f)
        if err != nil {
            closeAll(sys,closers)
            return nil, nil, err
        }
        if closer != nil {
            closers = append(closers, closer)
        }

        sys.AddAppender("", a) // "" = attach to all level handlers
    }

    return sys, closers, nil
}



func applyOptionDefaults(o Options) Options {
    if o.MinLevel == ""     { o.MinLevel = "info" }
    if o.Buffer == 0        { o.Buffer = 4096 }
    if o.BatchSize == 0     { o.BatchSize = 256 }
    if o.FlushInterval == 0 { o.FlushInterval = 100 }
	if o.MinSkip ==0        {o.MinSkip=4}

    for i := range o.Appenders {
        // rotating_file zero values
        if o.Appenders[i].Type == "rotating_file" {
            if o.Appenders[i].MaxSize == 0    { o.Appenders[i].MaxSize = 100 }   // 100 MB
            if o.Appenders[i].MaxAge == 0     { o.Appenders[i].MaxAge = 30 }     // 30 days
            if o.Appenders[i].MaxBackups == 0 { o.Appenders[i].MaxBackups = 5 }
        }
        // formatter zero value
        if o.Appenders[i].Formatter == "" { o.Appenders[i].Formatter = "text" }
    }
    return o
}



func validateAppenderOption(ao AppenderOption) error {
    switch ao.Type {
    case "console":
        // no extra fields required
    case "file", "rotating_file":
        if ao.Path == "" {
            return fmt.Errorf("mlog: appender type %q requires Path to be set", ao.Type)
        }
    default:
        return fmt.Errorf("mlog: unknown appender type %q (want: console|file|rotating_file)", ao.Type)
    }
    if ao.Formatter != "" && ao.Formatter != "text" && ao.Formatter != "json" {
        return fmt.Errorf("mlog: unknown formatter %q (want: text|json)", ao.Formatter)
    }
    return nil
}

// buildAppenderFromOption constructs one appender from an AppenderOption.
// Mirrors buildAppender() but takes AppenderOption instead of appenderConfig.
func buildAppenderFromOption(ao AppenderOption, f formatter.LogFormatter) (appender.LogAppender, func() error, error) {
    switch ao.Type {
    case "console":
        return appender.NewConsoleAppender(f), nil, nil

    case "file":
        os.MkdirAll(filepath.Dir(ao.Path), 0755)
        fa, err := appender.NewFileAppender(ao.Path, f)
        if err != nil {
            return nil, nil, fmt.Errorf("mlog: cannot open log file %q: %w", ao.Path, err)
        }
        return fa, fa.CloseFile, nil

    case "rotating_file":
        os.MkdirAll(filepath.Dir(ao.Path), 0755)
        fa := appender.NewRotatingFileAppender(
            ao.Path, ao.MaxSize, ao.MaxAge, ao.MaxBackups,
            ao.LocalTime, ao.Compress, f,
        )
        return fa, fa.CloseFile, nil
    }
   return nil, nil, fmt.Errorf("mlog: unknown appender type %q", ao.Type)
}
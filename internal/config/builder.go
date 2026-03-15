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

func Build(cfg *LoggerConfig) (*logs.Logger, []func() error, error) {
	var closers []func() error
	level, ok := logmsg.ParseLevel(cfg.MinLevel)
	if !ok {
		return nil, closers, fmt.Errorf("Config: not valid log level: %q", cfg.MinLevel)
	}
	system := logs.NewLogger(cfg.Buffer, level, cfg.BatchSize, time.Duration(cfg.FlushInterval)*time.Millisecond)

	for _, lv := range cfg.Levels {
		for _, ac := range lv.Appenders {

			f, err := buildFormatter(ac.Formatter)
			if err != nil {
				for _, c := range closers {
					c()
				}
				return nil, nil, err
			}

			a, closer, err := buildAppender(ac, f)
			if err != nil {
				for _, c := range closers {
					c()
				}
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
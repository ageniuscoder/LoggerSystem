package config

type LoggerConfig struct {
	Levels        []levelConfig `json:"levels" validate:"required,min=1,dive"`
	Buffer        int           `json:"buffer" validate:"gte=0,lte=100000"`
	MinLevel      string        `json:"min_level" validate:"oneof=debug info warning error fatal"`
	BatchSize     int           `json:"batch_size" validate:"gte=1,lte=10000"`
	MinSkip       int           `json:"min_skip" validate:"gte=1,lte=10"`
	FlushInterval int           `json:"flush_interval" validate:"gte=10,lte=60000"` //in milisecond
}

type formatterConfig struct {
	Type string `json:"type" validate:"required,oneof=json text"`
}

type appenderConfig struct {
	Name      string          `json:"name"        validate:"required"`
	Type      string          `json:"type"        validate:"required,oneof=console file rotating_file"`
	Formatter formatterConfig `json:"formatter"   validate:"required"`

	// file + rotating_file
	Path string `json:"path,omitempty"        validate:"required_if=Type file,required_if=Type rotating_file"`

	// rotating_file only
	MaxSize    int  `json:"max_size,omitempty"    validate:"required_if=Type rotating_file,omitempty,gt=0"`
	MaxAge     int  `json:"max_age,omitempty"     validate:"omitempty,gt=0"`
	MaxBackups int  `json:"max_backups,omitempty" validate:"omitempty,gt=0"`
	LocalTime  bool `json:"local_time,omitempty"`
	Compress   bool `json:"compress,omitempty"`
}

type levelConfig struct {
	Level     string           `json:"level" validate:"required,oneof=debug info warning error fatal"`
	Appenders []appenderConfig `json:"appenders" validate:"required,min=1,dive"`
}

//for user convenience

// Options is the programmatic config used by mlog.New() and functional options.
// It is the code equivalent of logger.json — no file needed.
type Options struct {
	MinLevel      string // "debug" | "info" | "warning" | "error" | "fatal"
	Buffer        int    // async channel capacity
	BatchSize     int    // flush after this many messages
	FlushInterval int    // flush after this many milliseconds
	MinSkip       int    // Min skip for caller
	Appenders     []AppenderOption
}

// AppenderOption describes one appender inside Options.
// When built via BuildFromOptions, every appender attaches to all level handlers.
type AppenderOption struct {
	Type      string // "console" | "file" | "rotating_file"
	Formatter string // "text" | "json"

	// Required when Type is "file" or "rotating_file"
	Path string

	// rotating_file only — sensible zero-value defaults applied in BuildFromOptions
	MaxSize    int  // megabytes per file before rotation
	MaxAge     int  // days to keep old files
	MaxBackups int  // max number of old files to keep
	LocalTime  bool // use local time in rotation timestamps
	Compress   bool // gzip rotated files
}

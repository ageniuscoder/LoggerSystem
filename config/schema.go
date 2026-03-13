package config

type LoggerConfig struct {
	Levels        []levelConfig `json:"levels" validate:"required,min=1,dive"`
	Buffer        int           `json:"buffer" validate:"gte=0,lte=100000"`
	MinLevel      string        `json:"min_level" validate:"oneof=debug info warning error"`
	BatchSize     int           `json:"batch_size" validate:"gte=1,lte=10000"`
	FlushInterval int           `json:"flush_interval" validate:"gte=10,lte=60000"` //in milisecond
}

type formatterConfig struct {
	Name string `json:"name" validate:"required"`
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
	Level     string           `json:"level" validate:"required,oneof=debug info warning error"`
	Appenders []appenderConfig `json:"appenders" validate:"required,min=1,dive"`
}
package start

import (
	"fmt"
	"logger/config"
	logs "logger/logger"
)

func Run(path string) (*logs.Logger,[]func() error,error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil,nil,fmt.Errorf("Logger Run failed: %w",err)
	}

	system, closers, err := config.Build(cfg)
	if err != nil {
		return nil,nil,fmt.Errorf("Logger Run failed: %w",err)
	}

	return system,closers,nil
}
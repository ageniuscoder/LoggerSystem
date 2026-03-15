package mlog

import (
	"fmt"

	"github.com/ageniuscoder/mlog/internal/config"
	logs "github.com/ageniuscoder/mlog/internal/logger"
	"github.com/ageniuscoder/mlog/internal/logmsg"
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

func ShutDown(closers []func() error,system *logs.Logger){
	system.Shutdown()
	for _,c:=range closers{
		c()
	}
}

func M(key string,val any) logmsg.Field{
	return logmsg.M(key,val)
}
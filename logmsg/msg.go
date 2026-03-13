package logmsg

import (
	"path/filepath"
	"runtime"
	"time"
)

type LogMsg struct {
	Timestamp time.Time
	Level LogLevel
	Content string
	Fields []Field
	File string
	Line int
}
/*
runtime.Caller in Go is used to get information about the function call stack, such as:
program counter
file name
line number
*/

func NewLogMsg(level LogLevel,content string, fields []Field) *LogMsg{
	_,file,line,ok:=runtime.Caller(2)
	if ok{
		file=filepath.Base(file)
	}
	return &LogMsg{
		Timestamp: time.Now(),
		Level: level,
		Content: content,
		Fields: fields,
		File: file,
		Line: line,
	}
}

func (lm *LogMsg) GetTimestamp() string{
	st:=lm.Timestamp.Format("2006-01-02 15:04:05")
	return st
}

func (lm *LogMsg) GetContent() string{
	return lm.Content
}

func (lm *LogMsg) GetLevel() string{
	return lm.Level.ToStr()
}
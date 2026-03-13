package logmsg

import (
	"path/filepath"
	"runtime"
	"sync"
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
var msgPool=sync.Pool{
	New: func() any {
		return &LogMsg{}
	},
}

func getMsgPool() *LogMsg{
	return msgPool.Get().(*LogMsg)
}

func PutMsgPool(m *LogMsg){
	m.Fields=m.Fields[:0]
	m.Content = ""         
    m.File    = ""
    m.Line    = 0
	msgPool.Put(m)
}
func NewLogMsg(level LogLevel,content string, fields []Field) *LogMsg{
	_,file,line,ok:=runtime.Caller(3)
	if ok{
		file=filepath.Base(file)
	}
	m:=getMsgPool()
	m.Timestamp=time.Now()
	m.Level=level
	m.Content=content
	m.Fields=fields
	m.File=file
	m.Line=line
	return m
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
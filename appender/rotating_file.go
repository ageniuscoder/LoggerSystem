package appender

import (
	"bytes"
	"logger/formatter"
	"logger/logmsg"

	"gopkg.in/natefinch/lumberjack.v2"
)

type RotatingFileAppender struct {
	writer *lumberjack.Logger
	formatter formatter.LogFormatter
}

func NewRotatingFileAppender(path string,maxSize int,maxAge int,maxBackups int,localTime bool,compress bool,formatter formatter.LogFormatter) *RotatingFileAppender{
	return &RotatingFileAppender{
		writer: &lumberjack.Logger{
			Filename: path,
			MaxSize: maxSize,
			MaxAge: maxAge,
			MaxBackups: maxBackups,
			LocalTime: localTime,
			Compress: compress,
		},
		formatter: formatter,
	}
}

func (r *RotatingFileAppender) AppendMsg(msg *logmsg.LogMsg) error {
    line := r.formatter.Format(msg)
    var buf bytes.Buffer
    buf.Grow(len(line) + 1)
    buf.WriteString(line)
    buf.WriteByte('\n')
    _, err := r.writer.Write(buf.Bytes())
    return err
}

func (r *RotatingFileAppender) AppendBatch(msgs []*logmsg.LogMsg) error {
	if len(msgs)==0{
		return nil
	}

	var buf bytes.Buffer
	buf.Grow(len(msgs)*80)

	for _,m:=range msgs{
		buf.WriteString(r.formatter.Format(m))
		buf.WriteByte('\n')
	}
	_,err:=r.writer.Write(buf.Bytes())
	return err
}

func (r *RotatingFileAppender) CloseFile() error {
	return r.writer.Close()
}
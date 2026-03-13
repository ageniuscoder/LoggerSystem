package formatter

import (
	"logger/logmsg"
	"strconv"
	"sync"
)

type LogFormatter interface {
	Format(msg *logmsg.LogMsg) string
}

type TextFormatter struct{
}

func NewTextFormatter() *TextFormatter{
	return &TextFormatter{
	}
}

/*
sync.Pool is for:

temporary objects
high allocation rate
short-lived objects

It reduces:

allocations
garbage collection
memory pressure
*/

var bufPool=sync.Pool{    //here return type is pointer so always use Byte slice by derefrencing it
	New: func() any {
		b:=make([]byte,0,256)
		return &b
	},
}

func getBuffer() *[]byte{
	return bufPool.Get().(*[]byte)
}

func putBuffer(b *[]byte) {
	*b = (*b)[:0]   // reset length
	bufPool.Put(b)
}

func (tf *TextFormatter) Format(msg *logmsg.LogMsg) string {
  bf := getBuffer()
  *bf = append(*bf, '[')
  *bf = append(*bf, msg.GetLevel()...)
  *bf = append(*bf, "]- "...)
  *bf = append(*bf, msg.GetTimestamp()...)
  *bf = append(*bf, ' ')
  *bf = append(*bf, msg.File...)
  *bf = append(*bf, ':')
  *bf = strconv.AppendInt(*bf, int64(msg.Line), 10)
  *bf = append(*bf, ": "...)
  *bf = append(*bf, msg.Content...)
  for _, f := range msg.Fields {
    *bf = append(*bf, " | "...)
    *bf = append(*bf, f.Key...)
    *bf = append(*bf, '=')
    *bf = f.AppendTextValue(*bf)
  }
  result := string(*bf)
  putBuffer(bf)
  return result
}


type JsonFormatter struct{
}

func NewJsonFormatter() *JsonFormatter{
	return &JsonFormatter{
	}
}

func (jf *JsonFormatter) Format(msg *logmsg.LogMsg) string {  // ` ` used to process raw strings special char are not treated as special in it
	bf:=getBuffer()
	*bf=append(*bf,  `{"timestamp":"`...)
	*bf=append(*bf,  msg.GetTimestamp()...)
	*bf=append(*bf,  `","level":"`...)
	*bf=append(*bf,  msg.GetLevel()...)
	*bf=append(*bf,  `","caller":"`...)
	*bf=append(*bf,  msg.File...)
	*bf=append(*bf,   ':')
	*bf = strconv.AppendInt(*bf, int64(msg.Line), 10)
	*bf = append(*bf, `","msg":`...)
	*bf = logmsg.AppendJSONString(*bf, msg.Content)

	// Write every structured field inline
	for _, f := range msg.Fields {
		*bf = append(*bf, ',')
		*bf = f.AppendJSON(*bf)
	}
	*bf = append(*bf, '}')

	result:=string(*bf)

	putBuffer(bf)
	return result
}
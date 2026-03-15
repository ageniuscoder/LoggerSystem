package logs

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ageniuscoder/mlog/internal/appender"
	"github.com/ageniuscoder/mlog/internal/handler"
	"github.com/ageniuscoder/mlog/internal/logmsg"
)

//singleton
type Logger struct {
	handlers map[string]handler.LogHandler
	head handler.LogHandler
	logbuffer chan *logmsg.LogMsg
	wg sync.WaitGroup
	done chan struct{}
	minLevel logmsg.LogLevel
	batchSize int
	flushInterval time.Duration
	shutdownOnce sync.Once
	droppedCnt int64
}

//constructor based logger
func NewLogger(buffer int,minLevel logmsg.LogLevel,batchSize int,flushInterval time.Duration) *Logger{
	mp:=make(map[string]handler.LogHandler)
	mp["debug"]=handler.NewDebugHandler()
	mp["info"]=handler.NewInfoHandler()
	mp["warning"]=handler.NewWarningHandler()
	mp["error"]=handler.NewErrorHandler()
	minHead,ok:=mp[minLevel.ToStr()]
	if !ok{
		minHead=mp["debug"]
	}
	instance:=&Logger{
		handlers: mp,
		logbuffer: make(chan *logmsg.LogMsg,buffer),
		done: make(chan struct{}),
		head: minHead,
		minLevel: minLevel,
		batchSize: batchSize,
		flushInterval: flushInterval,
	}
	instance.handlers["debug"].SetNext(instance.handlers["info"])
	instance.handlers["info"].SetNext(instance.handlers["warning"])
	instance.handlers["warning"].SetNext(instance.handlers["error"])
	instance.handlers["error"].SetNext(nil)
	instance.wg.Add(1)
	go instance.batchWorker()
	return instance
}

func (l *Logger) flush(batch []*logmsg.LogMsg) {
    l.head.HandleBatch(batch)      // all appenders MUST be done here
    for _, m := range batch {
        logmsg.PutMsgPool(m)       // only safe because HandleBatch is synchronous
    }
    // Contract: appenders must not retain *LogMsg pointers after AppendBatch returns
}
// worker is the single background goroutine that drains the channel.
// Single worker = guaranteed FIFO ordering of all log messages.
//it logs in batches
func (l *Logger) batchWorker(){
	defer l.wg.Done()
	batch:=make([]*logmsg.LogMsg,0,l.batchSize)
	ticker:=time.NewTicker(l.flushInterval)
	defer ticker.Stop()
	// statsTicker:=time.NewTicker(1*time.Second)
	// defer statsTicker.Stop()
	for{
		select {
		case msg:=<-l.logbuffer:
			batch = append(batch, msg)
			if len(batch)>=l.batchSize {
				l.flush(batch)
				batch=batch[:0]
				ticker.Reset(l.flushInterval)
			}
		case <-ticker.C:
			if len(batch) > 0{
				l.flush(batch)
				batch=batch[:0]
			}
		// case <-statsTicker.C:
		// 	fmt.Fprintf(os.Stderr,"Dropped logs are: %v \n",l.GetDroppedLogsCnt())
		case <-l.done:     //logger is shutdown drain the buffer,bcz logbuffer is never closed
		for{
			select{
			case msg:=<-l.logbuffer:
				batch = append(batch, msg)
			default:
				if len(batch)>0{
					l.flush(batch)
					batch=batch[:0]
				}
				return
			}
		}
		}
	
	}
}


func (l *Logger) Shutdown(){  
	l.shutdownOnce.Do(func() {
		close(l.done)   // signals: reject new sends
		l.wg.Wait()
	})
}

func (l *Logger) AddAppender(level string,appender appender.LogAppender){
	if ap,ok:=l.handlers[level]; ok{
		ap.AddAppender(appender)
	}
}

func (l *Logger) log(level logmsg.LogLevel,msg string,fields []logmsg.Field){
	if level<l.minLevel{
		return
	}
	m:=logmsg.NewLogMsg(level,msg,fields,3)
	select{
	case <-l.done:
		//ignore silently if shutdown not panic
	case l.logbuffer<-m:
	default:
		//non blocking and keep track of dropped logs
		atomic.AddInt64(&l.droppedCnt,1)
		logmsg.PutMsgPool(m)
	}
	
}

func (l *Logger) Debug(msg string,fields ...logmsg.Field){
	l.log(logmsg.DEBUG,msg,fields)
}

func (l *Logger) Info(msg string, fields ...logmsg.Field) {
	l.log(logmsg.INFO, msg, fields)
}

func (l *Logger) Warning(msg string, fields ...logmsg.Field) {
	l.log(logmsg.WARNING, msg, fields)
}

func (l *Logger) Error(msg string, fields ...logmsg.Field) {
	l.log(logmsg.ERROR, msg, fields)
}
func (l *Logger) GetDroppedLogsCnt() int64{
	return atomic.LoadInt64(&l.droppedCnt)
}




package nsqd

import (
	"fmt"
	"github.com/absolute8511/nsq/internal/flume_log"
	"github.com/absolute8511/nsq/internal/levellogger"
	"time"
)

const (
	traceModule = "msgtracer"
)

type IMsgTracer interface {
	Start()
	TracePub(topic string, traceID uint64, msg *Message, diskOffset BackendOffset, currentCnt int64)
	// state will be READ_QUEUE, Start, Req, Fin, Timeout
	TraceSub(topic string, state string, traceID uint64, msg *Message, clientID string)
}

var nsqMsgTracer IMsgTracer

func SetRemoteMsgTracer(remote string) {
	if remote != "" {
		nsqMsgTracer = NewRemoteMsgTracer(remote)
	}
}

// just print the trace log
type LogMsgTracer struct {
	MID string
}

func (self *LogMsgTracer) Start() {
}

func (self *LogMsgTracer) TracePub(topic string, traceID uint64, msg *Message, diskOffset BackendOffset, currentCnt int64) {
	nsqLog.Logf("[TRACE] topic %v trace id %v: message %v put at offset: %v, current count: %v at time %v", topic, msg.TraceID,
		msg.ID, diskOffset, currentCnt, time.Now().UnixNano())
}

func (self *LogMsgTracer) TraceSub(topic string, state string, traceID uint64, msg *Message, clientID string) {
	nsqLog.Logf("[TRACE] topic %v trace id %v: message %v (offset: %v) consume state %v from client %v at time: %v, attempt: %v", topic, msg.TraceID,
		msg.ID, msg.offset, state, clientID, time.Now().UnixNano(), msg.Attempts)
}

// this tracer will send the trace info to remote server for each seconds
type RemoteMsgTracer struct {
	remoteAddr   string
	remoteLogger *flume_log.FlumeLogger
	localTracer  *LogMsgTracer
}

func NewRemoteMsgTracer(remote string) IMsgTracer {
	return &RemoteMsgTracer{
		remoteAddr:   remote,
		remoteLogger: flume_log.NewFlumeLoggerWithAddr(remote),
		localTracer:  &LogMsgTracer{},
	}
}

func (self *RemoteMsgTracer) Start() {
	self.localTracer.Start()
}

func (self *RemoteMsgTracer) Stop() {
	self.remoteLogger.Stop()
}

func (self *RemoteMsgTracer) TracePub(topic string, traceID uint64, msg *Message, diskOffset BackendOffset, currentCnt int64) {
	now := time.Now().UnixNano()
	detail := flume_log.NewDetailInfo(traceModule)
	detail.AddLogItem("msgid", msg.ID)
	detail.AddLogItem("traceid", msg.TraceID)
	detail.AddLogItem("topic", topic)
	detail.AddLogItem("timestamp", now)
	detail.AddLogItem("action", "PUB")

	l := fmt.Sprintf("[TRACE] topic %v trace id %v: message %v put at offset: %v, current count: %v at time %v", topic, msg.TraceID,
		msg.ID, diskOffset, currentCnt, now)
	err := self.remoteLogger.Info(l, detail)
	if err != nil || nsqLog.Level() >= levellogger.LOG_DEBUG {
		if err != nil {
			nsqLog.Warningf("send log to remote error: %v", err)
		}
		self.localTracer.TracePub(topic, traceID, msg, diskOffset, currentCnt)
	}
}

func (self *RemoteMsgTracer) TraceSub(topic string, state string, traceID uint64, msg *Message, clientID string) {
	now := time.Now().UnixNano()
	detail := flume_log.NewDetailInfo(traceModule)
	detail.AddLogItem("msgid", msg.ID)
	detail.AddLogItem("traceid", msg.TraceID)
	detail.AddLogItem("topic", topic)
	detail.AddLogItem("timestamp", now)
	detail.AddLogItem("action", state)

	l := fmt.Sprintf("[TRACE] topic %v trace id %v: message %v (offset: %v) consume state %v from client %v at time: %v, attempt: %v",
		topic, msg.TraceID, msg.ID, msg.offset, state, clientID, time.Now().UnixNano(), msg.Attempts)
	err := self.remoteLogger.Info(l, detail)
	if err != nil || nsqLog.Level() >= levellogger.LOG_DEBUG {
		if err != nil {
			nsqLog.Warningf("send log to remote error: %v", err)
		}
		self.localTracer.TraceSub(topic, state, traceID, msg, clientID)
	}
}

func init() {
	nsqMsgTracer = &LogMsgTracer{}
}

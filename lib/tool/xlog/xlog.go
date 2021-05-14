/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package xlog

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	log "github.com/qiniu/x/log.v7"
)

const logKey = "X-Log"
const tagKey = "X-Tag"
const uidKey = "X-Uid"
const reqidKey = "X-Reqid"

const (
	// Ldate ...
	Ldate = log.Ldate
	// Ltime ...
	Ltime = log.Ltime
	// Lmicroseconds ...
	Lmicroseconds = log.Lmicroseconds
	// Llongfile ...
	Llongfile = log.Llongfile
	// Lshortfile ...
	Lshortfile = log.Lshortfile
	// Lmodule ...
	Lmodule = log.Lmodule
	// Llevel 0(Debug), 1(Info), 2(Warn), 3(Error), 4(Panic), 5(Fatal)
	Llevel = log.Llevel
	// LstdFlags ...
	LstdFlags = log.LstdFlags
	// Ldefault ...
	Ldefault = log.Ldefault
)

const (
	// Ldebug ...
	Ldebug = log.Ldebug
	// Linfo ...
	Linfo = log.Linfo
	// Lwarn ...
	Lwarn = log.Lwarn
	// Lerror ...
	Lerror = log.Lerror
	// Lpanic ...
	Lpanic = log.Lpanic
	// Lfatal ...
	Lfatal = log.Lfatal
)

type reqIder interface {
	ReqId() string
}

type header interface {
	Header() http.Header
}

// Logger ...
type Logger struct {
	h     http.Header
	reqID string
}

var pid = uint32(os.Getpid())

var genReqID = defaultGenReqID

func defaultGenReqID() string {

	var b [12]byte
	binary.LittleEndian.PutUint32(b[:], pid)
	binary.LittleEndian.PutUint64(b[4:], uint64(time.Now().UnixNano()))
	return base64.URLEncoding.EncodeToString(b[:])
}

// GenReqID ...
func GenReqID() string {
	return genReqID()
}

// SetGenReqID ...
func SetGenReqID(f func() string) {

	if f == nil {
		f = defaultGenReqID
	}
	genReqID = f
}

// New ...
func New(w http.ResponseWriter, req *http.Request) *Logger {

	reqID := req.Header.Get(reqidKey)
	if reqID == "" {
		reqID = genReqID()
		req.Header.Set(reqidKey, reqID)
	}
	h := w.Header()
	h.Set(reqidKey, reqID)

	return &Logger{h, reqID}
}

// NewWithReq ...
func NewWithReq(req *http.Request) *Logger {

	reqID := req.Header.Get(reqidKey)
	if reqID == "" {
		reqID = genReqID()
		req.Header.Set(reqidKey, reqID)
	}
	h := http.Header{reqidKey: []string{reqID}}

	return &Logger{h, reqID}
}

// NewWith Born a logger with:
// 	1. provided req id (if @a is reqIder)
// 	2. provided header (if @a is header)
//	3. **DUMMY** trace recorder (if @a cannot record)
//
func NewWith(a interface{}) *Logger {

	var h http.Header
	var reqID string

	if a == nil {
		reqID = genReqID()
	} else {
		l, ok := a.(*Logger)
		if ok {
			return l
		}
		reqID, ok = a.(string)
		if !ok {
			if g, ok := a.(reqIder); ok {
				reqID = g.ReqId()
			} else {
				panic("xlog.NewWith: unknown param")
			}
			if g, ok := a.(header); ok {
				h = g.Header()
			}
		}
	}
	if h == nil {
		h = http.Header{reqidKey: []string{reqID}}
	}
	return &Logger{h, reqID}
}

// NewDummy Born a logger with:
// 	1. new random req id
//	2. **DUMMY** trace recorder (will not record anything)
//
func NewDummy() *Logger {
	id := genReqID()
	return &Logger{
		h:     http.Header{reqidKey: []string{id}},
		reqID: id,
	}
}

// Spawn a child logger with:
// 	1. same req id with parent
// 	2. same trace recorder with parent(history compatibility consideration)
//
func (xlog *Logger) Spawn() *Logger {
	return &Logger{
		h: http.Header{
			reqidKey: []string{xlog.reqID},
		},
		reqID: xlog.reqID,
	}
}

// Xget ...
func (xlog *Logger) Xget() []string {
	return xlog.h[logKey]
}

// Xput ...
func (xlog *Logger) Xput(logs []string) {
	xlog.h[logKey] = append(xlog.h[logKey], logs...)
}

// Xlog ...
func (xlog *Logger) Xlog(v ...interface{}) {
	s := fmt.Sprint(v...)
	xlog.h[logKey] = append(xlog.h[logKey], s)
}

// Xlogf ...
func (xlog *Logger) Xlogf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	xlog.h[logKey] = append(xlog.h[logKey], s)
}

// Xtag ...
func (xlog *Logger) Xtag(v ...interface{}) {
	var ss = make([]string, 0)
	for _, e := range v {
		ss = append(ss, fmt.Sprint(e))
	}
	str := strings.Join(ss, ";")
	pre := xlog.h[tagKey]
	if len(pre) != 0 {
		str = pre[0] + ";" + str
	}
	xlog.h[tagKey] = []string{str}
}

// Xuid ...
func (xlog *Logger) Xuid(uid uint32) {
	s := fmt.Sprint(uid)
	xlog.h[uidKey] = []string{s}
}

/*
* 用法示意：

	func Foo(log xlog.*Logger) {
		...
		now := time.Now()
		err := longtimeOperation()
		log.Xprof("longtimeOperation", now, err)
		...
	}
*/

// Xprof ...
func (xlog *Logger) Xprof(modFn string, start time.Time, err error) {
	xlog.Xprof2(modFn, time.Since(start), err)
}

// Xprof2 ...
func (xlog *Logger) Xprof2(modFn string, dur time.Duration, err error) {

	const maxErrorLen = 32
	durMs := dur.Nanoseconds() / 1e6
	if durMs > 0 {
		modFn += ":" + strconv.FormatInt(durMs, 10)
	}
	if err != nil {
		msg := err.Error()
		if len(msg) > maxErrorLen {
			msg = msg[:maxErrorLen]
		}
		modFn += "/" + msg
	}
	xlog.h[logKey] = append(xlog.h[logKey], modFn)
}

/*
* 用法示意：

	func Foo(log xlog.*Logger) (err error) {
		defer log.Xtrack("Foo", time.Now(), &err)
		...
	}

	func Bar(log xlog.*Logger) {
		defer log.Xtrack("Bar", time.Now(), nil)
		...
	}
*/

// Xtrack ...
func (xlog *Logger) Xtrack(modFn string, start time.Time, errTrack *error) {
	var err error
	if errTrack != nil {
		err = *errTrack
	}
	xlog.Xprof(modFn, start, err)
}

// ReqID ...
func (xlog *Logger) ReqID() string {
	return xlog.reqID
}

// Header ...
func (xlog *Logger) Header() http.Header {
	return xlog.h
}

// SetLoggerLevel ...
func (xlog *Logger) SetLoggerLevel(level int) {
	log.Std.SetOutputLevel(level)
}

// Print calls Output to print to the standard Logger.
// Arguments are handled in the manner of fmt.Print.
func (xlog *Logger) Print(v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Linfo, 2, fmt.Sprint(v...))
}

// Printf calls Output to print to the standard Logger.
// Arguments are handled in the manner of fmt.Printf.
func (xlog *Logger) Printf(format string, v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Linfo, 2, fmt.Sprintf(format, v...))
}

// Println calls Output to print to the standard Logger.
// Arguments are handled in the manner of fmt.Println.
func (xlog *Logger) Println(v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Linfo, 2, fmt.Sprintln(v...))
}

// Debugf ...
func (xlog *Logger) Debugf(format string, v ...interface{}) {
	if log.Ldebug < log.Std.Level {
		return
	}
	log.Std.Output(xlog.reqID, log.Ldebug, 2, fmt.Sprintf(format, v...))
}

// Debug ...
func (xlog *Logger) Debug(v ...interface{}) {
	if log.Ldebug < log.Std.Level {
		return
	}
	log.Std.Output(xlog.reqID, log.Ldebug, 2, fmt.Sprintln(v...))
}

// Infof ...
func (xlog *Logger) Infof(format string, v ...interface{}) {
	if log.Linfo < log.Std.Level {
		return
	}
	log.Std.Output(xlog.reqID, log.Linfo, 2, fmt.Sprintf(format, v...))
}

// Info ...
func (xlog *Logger) Info(v ...interface{}) {
	if log.Linfo < log.Std.Level {
		return
	}
	log.Std.Output(xlog.reqID, log.Linfo, 2, fmt.Sprintln(v...))
}

// Warnf ...
func (xlog *Logger) Warnf(format string, v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Lwarn, 2, fmt.Sprintf(format, v...))
}

// Warn ...
func (xlog *Logger) Warn(v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Lwarn, 2, fmt.Sprintln(v...))
}

// Errorf ...
func (xlog *Logger) Errorf(format string, v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Lerror, 2, fmt.Sprintf(format, v...))
}

// Error ...
func (xlog *Logger) Error(v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Lerror, 2, fmt.Sprintln(v...))
}

// Fatal is equivalent to Print() followed by a call to os.Exit(1).
func (xlog *Logger) Fatal(v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Lfatal, 2, fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf is equivalent to Printf() followed by a call to os.Exit(1).
func (xlog *Logger) Fatalf(format string, v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Lfatal, 2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Fatalln is equivalent to Println() followed by a call to os.Exit(1).
func (xlog *Logger) Fatalln(v ...interface{}) {
	log.Std.Output(xlog.reqID, log.Lfatal, 2, fmt.Sprintln(v...))
	os.Exit(1)
}

// -----------------------------------------

// Panic is equivalent to Print() followed by a call to panic().
func (xlog *Logger) Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	log.Std.Output(xlog.reqID, log.Lpanic, 2, s)
	panic(s)
}

// Panicf is equivalent to Printf() followed by a call to panic().
func (xlog *Logger) Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	log.Std.Output(xlog.reqID, log.Lpanic, 2, s)
	panic(s)
}

// Panicln is equivalent to Println() followed by a call to panic().
func (xlog *Logger) Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	log.Std.Output(xlog.reqID, log.Lpanic, 2, s)
	panic(s)
}

// Stack ...
func (xlog *Logger) Stack(v ...interface{}) {
	s := fmt.Sprint(v...)
	s += "\n"
	buf := make([]byte, 1024*1024)
	n := runtime.Stack(buf, true)
	s += string(buf[:n])
	s += "\n"
	log.Std.Output(xlog.reqID, log.Lerror, 2, s)
}

// SingleStack ...
func (xlog *Logger) SingleStack(v ...interface{}) {
	s := fmt.Sprint(v...)
	s += "\n"
	buf := make([]byte, 1024*1024)
	n := runtime.Stack(buf, false)
	s += string(buf[:n])
	s += "\n"
	log.Std.Output(xlog.reqID, log.Lerror, 2, s)
}

// Debugf ...
func Debugf(reqID string, format string, v ...interface{}) {
	if log.Ldebug < log.Std.Level {
		return
	}
	log.Std.Output(reqID, log.Ldebug, 2, fmt.Sprintf(format, v...))
}

// Debug ...
func Debug(reqID string, v ...interface{}) {
	if log.Ldebug < log.Std.Level {
		return
	}
	log.Std.Output(reqID, log.Ldebug, 2, fmt.Sprintln(v...))
}

// Infof ...
func Infof(reqID string, format string, v ...interface{}) {
	if log.Linfo < log.Std.Level {
		return
	}
	log.Std.Output(reqID, log.Linfo, 2, fmt.Sprintf(format, v...))
}

// Info ...
func Info(reqID string, v ...interface{}) {
	if log.Linfo < log.Std.Level {
		return
	}
	log.Std.Output(reqID, log.Linfo, 2, fmt.Sprintln(v...))
}

// Warnf ...
func Warnf(reqID string, format string, v ...interface{}) {
	log.Std.Output(reqID, log.Lwarn, 2, fmt.Sprintf(format, v...))
}

// Warn ...
func Warn(reqID string, v ...interface{}) {
	log.Std.Output(reqID, log.Lwarn, 2, fmt.Sprintln(v...))
}

// Errorf ...
func Errorf(reqID string, format string, v ...interface{}) {
	log.Std.Output(reqID, log.Lerror, 2, fmt.Sprintf(format, v...))
}

// Error ...
func Error(reqID string, v ...interface{}) {
	log.Std.Output(reqID, log.Lerror, 2, fmt.Sprintln(v...))
}

// Fatal is equivalent to Print() followed by a call to os.Exit(1).
func Fatal(reqID string, v ...interface{}) {
	log.Std.Output(reqID, log.Lfatal, 2, fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf is equivalent to Printf() followed by a call to os.Exit(1).
func Fatalf(reqID string, format string, v ...interface{}) {
	log.Std.Output(reqID, log.Lfatal, 2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Fatalln is equivalent to Println() followed by a call to os.Exit(1).
func Fatalln(reqID string, v ...interface{}) {
	log.Std.Output(reqID, log.Lfatal, 2, fmt.Sprintln(v...))
	os.Exit(1)
}

// -----------------------------------------

// Panic is equivalent to Print() followed by a call to panic().
func Panic(reqID string, v ...interface{}) {
	s := fmt.Sprint(v...)
	log.Std.Output(reqID, log.Lpanic, 2, s)
	panic(s)
}

// Panicf is equivalent to Printf() followed by a call to panic().
func Panicf(reqID string, format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	log.Std.Output(reqID, log.Lpanic, 2, s)
	panic(s)
}

// Panicln is equivalent to Println() followed by a call to panic().
func Panicln(reqID string, v ...interface{}) {
	s := fmt.Sprintln(v...)
	log.Std.Output(reqID, log.Lpanic, 2, s)
	panic(s)
}

// Stack ...
func Stack(reqID string, v ...interface{}) {
	s := fmt.Sprint(v...)
	s += "\n"
	buf := make([]byte, 1024*1024)
	n := runtime.Stack(buf, true)
	s += string(buf[:n])
	s += "\n"
	log.Std.Output(reqID, log.Lerror, 2, s)
}

// SingleStack ...
func SingleStack(reqID string, v ...interface{}) {
	s := fmt.Sprint(v...)
	s += "\n"
	buf := make([]byte, 1024*1024)
	n := runtime.Stack(buf, false)
	s += string(buf[:n])
	s += "\n"
	log.Std.Output(reqID, log.Lerror, 2, s)
}

// SetOutput ...
func SetOutput(w io.Writer) {
	log.SetOutput(w)
}

// SetFlags ...
func SetFlags(flag int) {
	log.SetFlags(flag)
}

// SetOutputLevel ...
func SetOutputLevel(lvl int) {
	log.SetOutputLevel(lvl)
}

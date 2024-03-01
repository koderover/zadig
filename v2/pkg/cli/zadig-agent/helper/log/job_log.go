/*
Copyright 2023 The KodeRover Authors.

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

package log

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

type JobLogger struct {
	mu               sync.Mutex
	writer           io.Writer
	logger           *zap.SugaredLogger
	lumberjackLogger *lumberjack.Logger
	logPath          string
	isClosed         bool
}

func NewJobLogger(logfile string) *JobLogger {
	file, err := os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		Panicf("failed to open log file: %s", err)
	}

	cfg := &Config{
		Level:      "debug",
		Filename:   logfile,
		SendToFile: true,
		NoCaller:   true,
		NoLogLevel: true,
	}

	jobLogger := &JobLogger{
		mu:      sync.Mutex{},
		writer:  file,
		logPath: logfile,
	}
	jobLogger.logger, jobLogger.lumberjackLogger = InitJobLogger(cfg)
	return jobLogger
}

func (l *JobLogger) Printf(format string, a ...any) {
	if l.logger == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := fmt.Fprintf(l.writer, format, a...)
	if err != nil {
		l.logger.Errorf("Failed to write to log: %v\n", err)
	}
}

func (l *JobLogger) Println(args ...interface{}) {
	if l.logger == nil {
		return
	}

	raw := fmt.Sprintln(args...)

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := io.WriteString(l.writer, raw); err != nil {
		Errorf("Failed to write to log: %v\n", err)
	}
}

func (l *JobLogger) Debugf(args ...interface{}) {
	if l.logger == nil {
		return
	}

	raw := fmt.Sprint(args...)

	l.logger.Debug(raw)
}

func (l *JobLogger) Infof(args ...interface{}) {
	if l.logger == nil {
		return
	}

	raw := fmt.Sprint(args...)

	l.logger.Infof(raw)
}

func (l *JobLogger) Warnf(args ...interface{}) {
	if l.logger == nil {
		return
	}

	raw := fmt.Sprint(args...)

	l.logger.Warnf(raw)
}

func (l *JobLogger) Errorf(args ...interface{}) {
	if l.logger == nil {
		return
	}

	raw := fmt.Sprint(args...)

	l.logger.Errorf(raw)
}

func (l *JobLogger) Fatalf(args ...interface{}) {
	if l.logger == nil {
		return
	}

	raw := fmt.Sprint(args...)

	l.logger.Fatalf(raw)
}

func (l *JobLogger) Panicf(args ...interface{}) {
	if l.logger == nil {
		return
	}

	raw := fmt.Sprint(args...)

	l.logger.Panicf(raw)
}

func (l *JobLogger) Write(p []byte) {
	if l.logger == nil {
		return
	}

	raw := string(p)

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := io.WriteString(l.writer, raw); err != nil {
		Errorf("Failed to write to log: %v\n", err)
	}
}

// 读取文件内容到字符串
func readFileToString(filePath string) (string, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadByRowNum TODO: 需要优化，按行读取io过多效率低
func (l *JobLogger) ReadByRowNum(offset, curNum, num int64) ([]byte, int64, int64, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	file, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		Panicf("failed to open log file: %s", err)
	}
	defer file.Close()

	// Seek to the beginning of the file
	_, err = file.Seek(offset, 0)
	if err != nil {
		return nil, 0, 0, false, fmt.Errorf("failed to seek to %v offset of the file: %v", offset, err)
	}

	// Create a buffered reader
	reader := bufio.NewReader(file)

	// Counter to track the current line number
	lineCount := int64(0)

	// Buffer to store the read lines
	var resultBuffer bytes.Buffer

	// Read the file line by line until reaching the specified line count or end of file
	for lineCount < num {
		line, err := reader.ReadString('\n')
		if err == nil || err == io.EOF {
			// If the current line number is within the specified range, append the line data to the result buffer
			resultBuffer.WriteString(line)
			offset += int64(len(line))
			lineCount++

			if err == io.EOF {
				return resultBuffer.Bytes(), offset, lineCount, true, nil
			}
		} else {
			return resultBuffer.Bytes(), offset, lineCount, false, fmt.Errorf("failed to read log line: %v", err)
		}
	}
	return resultBuffer.Bytes(), offset, lineCount, false, nil
}

func (l *JobLogger) GetLogfilePath() string {
	return l.logPath
}

func (l *JobLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isClosed {
		return
	}

	closer, ok := l.writer.(io.Closer)
	if ok {
		if err := closer.Close(); err != nil {
			Errorf("failed to close writer, error: %s", err)
		}
	}

	if l.lumberjackLogger != nil {
		if err := l.lumberjackLogger.Close(); err != nil {
			Errorf("failed to close lumberjack logger, error: %s", err)
		}
	}

	err := l.logger.Sync()
	if err != nil {
		Errorf("failed to sync job logger, error: %s", err)
	}

	l.isClosed = true
}

func (l *JobLogger) Sync() {
	l.mu.Lock()
	defer l.mu.Unlock()

	err := l.logger.Sync()
	if err != nil {
		Errorf("failed to sync job logger, error: %s", err)
	}
}

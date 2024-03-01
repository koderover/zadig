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
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
)

var logger *zap.Logger
var simpleLogger *zap.SugaredLogger

type Config struct {
	Level       string
	SendToFile  bool
	Filename    string
	NoCaller    bool
	NoLogLevel  bool
	Development bool
	MaxSize     int // megabytes
	MaxAge      int // days
	MaxBackups  int
}

func init() {
	path, err := config.GetAgentLogPath()
	if err != nil {
		panic(err)
	}

	cfg := &Config{
		Level:      "debug",
		Filename:   filepath.Join(path, "zadig-agent.log"),
		SendToFile: true,
	}

	var l = new(zapcore.Level)
	err = l.UnmarshalText([]byte(cfg.Level))
	if err != nil {
		panic(err)
	}

	consoleSyncer := zapcore.AddSync(os.Stdout)
	consoleEncoder := getConsoleEncoder(cfg)
	consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, l)

	var opts []zap.Option
	opts = append(opts, zap.AddStacktrace(zap.DPanicLevel))
	if !cfg.NoCaller {
		opts = append(opts, zap.AddCaller())
	}
	if cfg.Development {
		opts = append(opts, zap.Development())
	}

	core := consoleCore
	if cfg.SendToFile {
		lumberjackLogger := getLumberjackLogger(cfg.Filename, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
		fileSyncer := zapcore.AddSync(lumberjackLogger)
		fileEncoder := getJSONEncoder(cfg)
		fileCore := zapcore.NewCore(fileEncoder, fileSyncer, l)

		core = zapcore.NewTee(consoleCore, fileCore)
	}

	logger = zap.New(core, opts...)
	simpleLogger = logger.WithOptions(zap.AddCallerSkip(1)).Sugar()
}

func InitJobLogger(cfg *Config) (*zap.SugaredLogger, *lumberjack.Logger) {
	var l = new(zapcore.Level)
	err := l.UnmarshalText([]byte(cfg.Level))
	if err != nil {
		panic(err)
	}

	consoleCore := zapcore.NewNopCore()

	var opts []zap.Option
	if !cfg.NoCaller {
		opts = append(opts, zap.AddCaller())
	}
	if cfg.Development {
		opts = append(opts, zap.Development())
	}

	core := consoleCore
	var lumberjackLogger *lumberjack.Logger
	if cfg.SendToFile {
		lumberjackLogger = getLumberjackLogger(cfg.Filename, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
		fileSyncer := zapcore.AddSync(lumberjackLogger)
		fileEncoder := getJobLogEncoder(cfg)
		fileCore := zapcore.NewCore(fileEncoder, fileSyncer, l)

		core = zapcore.NewTee(consoleCore, fileCore)
	}

	return zap.New(core, opts...).WithOptions(zap.AddCallerSkip(1)).Sugar(), lumberjackLogger
}

func getJobLogEncoder(cfg *Config) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()

	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = nil
	encoderConfig.EncodeDuration = nil

	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getJSONEncoder(cfg *Config) zapcore.Encoder {
	return getEncoder(cfg, true)
}

func getConsoleEncoder(cfg *Config) zapcore.Encoder {
	return getEncoder(cfg, false)
}

func lastNthIndexString(s string, sub string, index int) string {
	r := strings.Split(s, sub)
	if len(r) < index {
		return s
	}
	return strings.Join(r[len(r)-index:], "/")
}

func customCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(lastNthIndexString(caller.String(), "/", 3))
}

func getEncoder(cfg *Config, jsonFormat bool) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	encoderConfig.EncodeCaller = customCallerEncoder

	if cfg.NoLogLevel {
		encoderConfig.LevelKey = zapcore.OmitKey
	}

	if jsonFormat {
		return zapcore.NewJSONEncoder(encoderConfig)
	}

	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLumberjackLogger(filename string, maxSize, maxBackup, maxAge int) *lumberjack.Logger {
	// keep 10 backups (1GB) if not set to avoid run out of disk space
	if maxBackup == 0 {
		maxBackup = 10
	}
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxBackups: maxBackup,
		// MaxSize:    maxSize,
		// MaxAge:     maxAge,
	}

	return lumberJackLogger
}

func NopSugaredLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

func Logger() *zap.Logger {
	return getLogger()
}

func SugaredLogger() *zap.SugaredLogger {
	return getSugaredLogger()
}

func getLogger() *zap.Logger {
	if logger == nil {
		panic("Logger is not initialized yet!")
	}

	return logger
}

func getSugaredLogger() *zap.SugaredLogger {
	return getLogger().Sugar()
}

func GetSimpleLogger() *zap.SugaredLogger {
	if simpleLogger == nil {
		panic("Logger is not initialized yet!")
	}

	return simpleLogger
}

func GetLoggerFile() string {
	path, err := config.GetAgentLogPath()
	if err != nil {
		panic(err)
	}

	return filepath.Join(path, "zadig-agent.log")
}

func NewFileLogger(path string) *zap.Logger {
	lumberjackLogger := getLumberjackLogger(path, 0, 0, 0)
	fileSyncer := zapcore.AddSync(lumberjackLogger)
	fileEncoder := getJSONEncoder(&Config{})
	fileCore := zapcore.NewCore(fileEncoder, fileSyncer, zap.DebugLevel)
	return zap.New(fileCore)
}

func With(fields ...zap.Field) *zap.Logger {
	return getLogger().With(fields...)
}

func Debug(args ...interface{}) {
	GetSimpleLogger().Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	GetSimpleLogger().Debugf(format, args...)
}

func Info(args ...interface{}) {
	GetSimpleLogger().Info(args...)
}

func Infof(format string, args ...interface{}) {
	GetSimpleLogger().Infof(format, args...)
}

func Warning(args ...interface{}) {
	GetSimpleLogger().Warn(args...)
}

func Warningf(format string, args ...interface{}) {
	GetSimpleLogger().Warnf(format, args...)
}

func Warn(args ...interface{}) {
	GetSimpleLogger().Warn(args...)
}

func Warnf(format string, args ...interface{}) {
	GetSimpleLogger().Warnf(format, args...)
}

func Error(args ...interface{}) {
	GetSimpleLogger().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	GetSimpleLogger().Errorf(format, args...)
}

func DPanic(args ...interface{}) {
	GetSimpleLogger().DPanic(args...)
}

func DPanicf(format string, args ...interface{}) {
	GetSimpleLogger().DPanicf(format, args...)
}

func Panic(args ...interface{}) {
	GetSimpleLogger().Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	GetSimpleLogger().Panicf(format, args...)
}

func Fatal(args ...interface{}) {
	GetSimpleLogger().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	GetSimpleLogger().Fatalf(format, args...)
}

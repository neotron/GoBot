package core

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/jcelliott/lumber"
)

var log = lumber.NewConsoleLogger(lumber.DEBUG)

func init() {
	log.TimeFormat("2006-01-02 15:04:05.000")
	log.Prefix("GoBot")
}

func SetLogLevel(lvl int) {
	log.Level(lvl)
}

func IsLogInfo() bool {
	return log.IsInfo()
}
func IsLogDebug() bool {
	return log.IsDebug()
}
func IsLogWarn() bool {
	return log.IsWarn()
}
func IsLogError() bool {
	return log.IsError()
}

func LogDebugF(format string, v ...interface{}) {
	if log.IsDebug() {
		doLogF(log.Debug, format, v...)
	}
}

func LogInfoF(format string, v ...interface{}) {
	if log.IsInfo() {
		doLogF(log.Info, format, v...)
	}
}

func LogWarnF(format string, v ...interface{}) {
	if log.IsWarn() {
		doLogF(log.Warn, format, v...)
	}
}

func LogErrorF(format string, v ...interface{}) {
	if log.IsError() {
		doLogF(log.Error, format, v...)
	}
}

func LogFatalF(format string, v ...interface{}) {
	doLogF(log.Fatal, format, v...)
	os.Exit(2)
}

func LogDebug(v ...interface{}) {
	if log.IsDebug() {
		doLog(log.Debug, v...)
	}
}

func LogInfo(v ...interface{}) {
	if log.IsInfo() {
		doLog(log.Info, v...)
	}
}

func LogWarn(v ...interface{}) {
	if log.IsWarn() {
		doLog(log.Warn, v...)
	}
}

func LogError(v ...interface{}) {
	if log.IsError() {
		doLog(log.Error, v...)
	}
}

func LogFatal(v ...interface{}) {
	doLog(log.Fatal, v...)
	os.Exit(2)
}

func doLogF(logger func(format string, v ...interface{}), format string, v ...interface{}) {
	_, fn, line, _ := runtime.Caller(2)
	logger("%s:%d | %s", path.Base(fn), line, fmt.Sprintf(format, v...))
}

func doLog(logger func(format string, v ...interface{}), v ...interface{}) {
	_, fn, line, _ := runtime.Caller(2)
	logger("%s:%d | %s", path.Base(fn), line, fmt.Sprint(v...))
}

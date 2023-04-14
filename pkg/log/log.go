package log

import (
	"log"
	"os"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func init() {
	infoLogger = log.New(os.Stderr, "", 0)
	errorLogger = log.New(os.Stderr, "", 0)
}

func SetInfoToStdout() {
	infoLogger.SetOutput(os.Stdout)
}

func Infof(format string, v ...interface{}) {
	infoLogger.Printf(format, v...)
}

func Errorf(format string, v ...interface{}) {
	errorLogger.Printf(format, v...)
}

func Fatal(v ...any) {
	errorLogger.Fatal(v...)
}

func Panic(v ...any) {
	errorLogger.Panic(v...)
}

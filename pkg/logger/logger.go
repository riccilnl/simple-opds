package logger

import (
	"log"
	"os"
)

var (
	// Info 信息日志
	Info *log.Logger
	// Warning 警告日志
	Warning *log.Logger
	// Error 错误日志
	Error *log.Logger
)

// Init 初始化日志系统
func Init() {
	Info = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

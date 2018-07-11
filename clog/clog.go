package clog

import (
	"fmt"
	"log"
	"os"
)

//var logMutex sync.Mutex
var with_colors bool = true
var displayLogLevel = INFO

type LogLevel uint

type LogEntry struct {
	LogLevel
	Text string
}

func NewLogEntry(level LogLevel, text string) *LogEntry {
	return &LogEntry{level, text}
}

var LogQueue chan *LogEntry

const (
	DEBUGXX LogLevel = iota
	DEBUGX
	DEBUG
	INFO
	WARNING
	ERROR
)

var color_tags = [...]string{
	"\033[34mDEBUG++\033[0m",
	"\033[34mDEBUG+\033[0m",
	"\033[34mDEBUG\033[0m",
	"\033[90mINFO\033[0m",
	"\033[93mWARNING\033[0m",
	"\033[91mERROR\033[0m",
}

var plain_tags = [...]string{
	"DEBUG++",
	"DEBUG+",
	"DEBUG",
	"INFO",
	"WARNING",
	"ERROR",
}

var filelog *log.Logger
var logfile *os.File

func SetLogLevel(level LogLevel) {
	displayLogLevel = level
}

func SetLogFile(fname string) {
	logfile, err := os.Create(fname)
	if err != nil {
		Fatal("Cloud not open log file, %s", err)
	}
	filelog = log.New(logfile, "", log.LstdFlags)
	filelog.Println("**************** START *****************")
}

func Log(level LogLevel, format string, v ...interface{}) {

	if level < displayLogLevel {
		return
	}
	LogQueue <- NewLogEntry(level, fmt.Sprintf(format, v...))
}

func processLogQueue() {
	var tag string

	for {
		entry := <-LogQueue

		if with_colors {
			tag = color_tags[entry.LogLevel] + " "
		} else {
			tag = plain_tags[entry.LogLevel] + " "
		}

		log.Println(tag + entry.Text)
		if filelog != nil {
			filelog.Printf(plain_tags[entry.LogLevel] + " " + entry.Text)
		}
	}
}

func Terminate() {
	Log(INFO, "Terminating.")
	for len(LogQueue) > 0 {
	}
	if filelog != nil {
		filelog.Printf("**************** STOP *****************")
		logfile.Close()
	}
	os.Exit(1)
}

func Warning(format string, v ...interface{}) {
	Log(WARNING, format, v...)
}

func Error(format string, v ...interface{}) {
	Log(ERROR, format, v...)
}
func Fatal(format string, v ...interface{}) {
	Log(ERROR, format, v...)
	Terminate()
}

func Info(format string, v ...interface{}) {
	Log(INFO, format, v...)
}

func Debug(format string, v ...interface{}) {
	Log(DEBUG, format, v...)
}

func DebugX(format string, v ...interface{}) {
	Log(DEBUGX, format, v...)
}

func DebugXX(format string, v ...interface{}) {
	Log(DEBUGXX, format, v...)
}

func init() {
	LogQueue = make(chan *LogEntry, 32)
	go processLogQueue()
}

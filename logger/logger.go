package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var logWriter io.Writer

type CustomLogger struct{}

func (cl *CustomLogger) Write(p []byte) (n int, err error) {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		file = "???"
		line = 0
	}

	// Get just the filename without path
	filename := filepath.Base(file)

	// Format timestamp with milliseconds (3 digits)
	now := time.Now()
	timestamp := now.Format("2006/01/02 15:04:05.000")

	// Format with right-aligned 3-digit line number
	logLine := fmt.Sprintf("%s %s:%03d: %s", timestamp, filename, line, string(p))

	return logWriter.Write([]byte(logLine))
}

func Initialize() error {
	// Get the directory of the current executable
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(exePath)
	logPath := filepath.Join(exeDir, "application.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Set up multi-writer for both console and file
	logWriter = io.MultiWriter(os.Stdout, logFile)

	// Use custom logger
	customLogger := &CustomLogger{}
	log.SetOutput(customLogger)
	log.SetFlags(0) // Remove default formatting since we handle it in CustomLogger

	return nil
}

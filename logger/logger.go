package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
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
	return InitializeWithMode(false)
}

func InitializeWithMode(serviceMode bool) error {
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

	// For service mode, only write to file. For interactive mode, write to both console and file
	if serviceMode {
		logWriter = logFile
	} else {
		logWriter = io.MultiWriter(os.Stdout, logFile)
	}

	// Use custom logger
	customLogger := &CustomLogger{}
	log.SetOutput(customLogger)
	log.SetFlags(0) // Remove default formatting since we handle it in CustomLogger

	return nil
}

// ConvertToUTF8 converts byte slice to UTF-8 string, handling Japanese encoding if needed
func ConvertToUTF8(data []byte) string {
	// First check if it's already valid UTF-8
	if utf8.Valid(data) {
		return string(data)
	}
	
	// Try to decode as Shift_JIS (common on Windows)
	decoder := japanese.ShiftJIS.NewDecoder()
	result, _, err := transform.Bytes(decoder, data)
	if err == nil && utf8.Valid(result) {
		return string(result)
	}
	
	// If Shift_JIS fails, try EUC-JP
	decoder = japanese.EUCJP.NewDecoder()
	result, _, err = transform.Bytes(decoder, data)
	if err == nil && utf8.Valid(result) {
		return string(result)
	}
	
	// If all else fails, return as-is (may contain invalid UTF-8)
	return string(data)
}

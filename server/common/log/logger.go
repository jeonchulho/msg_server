package log

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type level string

const (
	debugLevel     level = "DEBUG"
	infoLevel      level = "INFO"
	warnLevel      level = "WARN"
	errorLevel     level = "ERROR"
	exceptionLevel level = "EXCEPTION"

	defaultLogFilePath   = "./logs/msg_server.log"
	defaultMaxSizeBytes  = 20 * 1024 * 1024
	envLogFilePath       = "LOG_FILE_PATH"
	envLogMaxSizeMB      = "LOG_MAX_SIZE_MB"
	envLogFormat         = "LOG_FORMAT"
	logFormatText        = "text"
	logFormatJSON        = "json"
	terminalColorReset   = "\033[0m"
	terminalColorGray    = "\033[90m"
	terminalColorGreen   = "\033[32m"
	terminalColorYellow  = "\033[33m"
	terminalColorRed     = "\033[31m"
	terminalColorMagenta = "\033[35m"
)

type logger struct {
	mu           sync.Mutex
	filePath     string
	maxSizeBytes int64
	format       string
	file         *os.File
}

var global = newLoggerFromEnv()

func newLoggerFromEnv() *logger {
	path := strings.TrimSpace(os.Getenv(envLogFilePath))
	if path == "" {
		path = defaultLogFilePath
	}

	maxSizeBytes := int64(defaultMaxSizeBytes)
	if raw := strings.TrimSpace(os.Getenv(envLogMaxSizeMB)); raw != "" {
		if sizeMB, err := strconv.Atoi(raw); err == nil && sizeMB > 0 {
			maxSizeBytes = int64(sizeMB) * 1024 * 1024
		}
	}
	format := strings.ToLower(strings.TrimSpace(os.Getenv(envLogFormat)))
	if format != logFormatJSON {
		format = logFormatText
	}

	return &logger{filePath: path, maxSizeBytes: maxSizeBytes, format: format}
}

func Debugf(format string, args ...any) {
	global.logf(debugLevel, format, args...)
}

func Infof(format string, args ...any) {
	global.logf(infoLevel, format, args...)
}

func Warnf(format string, args ...any) {
	global.logf(warnLevel, format, args...)
}

func Errorf(format string, args ...any) {
	global.logf(errorLevel, format, args...)
}

func Exceptionf(format string, args ...any) {
	global.logf(exceptionLevel, format, args...)
}

func (l *logger) logf(lv level, format string, args ...any) {
	ts := time.Now().Format(time.RFC3339Nano)
	caller := callerFuncName(3)
	message := fmt.Sprintf(format, args...)
	line := l.formatLine(ts, lv, caller, message)

	fmt.Fprintln(os.Stdout, colorForLevel(lv)+line+terminalColorReset)
	l.writeToFile(line + "\n")
}

func (l *logger) formatLine(ts string, lv level, caller, message string) string {
	if l.format == logFormatJSON {
		payload := map[string]string{
			"timestamp": ts,
			"level":     string(lv),
			"caller":    caller,
			"message":   message,
		}
		if b, err := json.Marshal(payload); err == nil {
			return string(b)
		}
	}
	return fmt.Sprintf("%s:%s:%s:%s", ts, lv, caller, message)
}

func (l *logger) writeToFile(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureOpen(); err != nil {
		fmt.Fprintf(os.Stderr, "logger open file error: %v\n", err)
		return
	}

	if err := l.rotateIfNeeded(int64(len(line))); err != nil {
		fmt.Fprintf(os.Stderr, "logger rotate error: %v\n", err)
		return
	}

	if _, err := l.file.WriteString(line); err != nil {
		fmt.Fprintf(os.Stderr, "logger write error: %v\n", err)
		return
	}
	_ = l.file.Sync()
}

func (l *logger) ensureOpen() error {
	if l.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.filePath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	l.file = f
	return nil
}

func (l *logger) rotateIfNeeded(incomingSize int64) error {
	if l.file == nil {
		return nil
	}
	stat, err := l.file.Stat()
	if err != nil {
		return err
	}
	if stat.Size()+incomingSize <= l.maxSizeBytes {
		return nil
	}

	if err := l.file.Sync(); err != nil {
		return err
	}
	if err := l.file.Close(); err != nil {
		return err
	}

	rotatedPath, err := nextRotatedPath(l.filePath)
	if err != nil {
		return err
	}
	if err := os.Rename(l.filePath, rotatedPath); err != nil {
		return err
	}

	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	l.file = f
	return nil
}

func nextRotatedPath(currentPath string) (string, error) {
	dir := filepath.Dir(currentPath)
	ext := filepath.Ext(currentPath)
	base := strings.TrimSuffix(filepath.Base(currentPath), ext)
	ts := time.Now().Format("20060102_150405")

	for index := 1; ; index++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%s_%d%s", base, ts, index, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

func callerFuncName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	fullName := fn.Name()
	parts := strings.Split(fullName, "/")
	if len(parts) == 0 {
		return fullName
	}
	return parts[len(parts)-1]
}

func colorForLevel(lv level) string {
	switch lv {
	case debugLevel:
		return terminalColorGray
	case infoLevel:
		return terminalColorGreen
	case warnLevel:
		return terminalColorYellow
	case errorLevel:
		return terminalColorRed
	case exceptionLevel:
		return terminalColorMagenta
	default:
		return terminalColorReset
	}
}

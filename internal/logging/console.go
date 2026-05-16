package logging

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	reset   = "\x1b[0m"
	dim     = "\x1b[2m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
	white   = "\x1b[37m"
)

var logFieldPattern = regexp.MustCompile(`\b([A-Za-z][A-Za-z0-9]*)=("[^"]*"|\S+)`)

type ConsoleWriter struct {
	out   io.Writer
	color bool
	mu    sync.Mutex
}

func NewConsoleWriter(out io.Writer) *ConsoleWriter {
	return &ConsoleWriter{
		out:   out,
		color: shouldColorConsole(),
	}
}

func (w *ConsoleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	lines := bytes.SplitAfter(p, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if _, err := fmt.Fprint(w.out, w.formatLine(string(line))); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *ConsoleWriter) formatLine(line string) string {
	if !w.color {
		return time.Now().Format("2006/01/02 15:04:05") + " " + line
	}
	timestamp := dim + cyan + time.Now().Format("2006/01/02 15:04:05") + reset
	return timestamp + " " + colorizeLogFields(line)
}

func shouldColorConsole() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if strings.EqualFold(os.Getenv("FEED_AI_NO_COLOR"), "1") || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return true
}

func colorizeLogFields(line string) string {
	return logFieldPattern.ReplaceAllStringFunc(line, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) != 2 {
			return match
		}
		key, value := parts[0], parts[1]
		color := colorForLogField(key, value)
		if color == "" {
			return match
		}
		return dim + key + "=" + reset + color + value + reset
	})
}

func colorForLogField(key, value string) string {
	switch key {
	case "method":
		return blue
	case "path", "mediaID", "filename":
		return cyan
	case "query":
		return magenta
	case "status":
		return colorForStatus(value)
	case "duration":
		return yellow
	case "bytes", "size", "items", "mediaItems", "comments", "activityItems":
		return white
	case "source":
		if strings.Contains(value, "cache") {
			return green
		}
		if strings.Contains(value, "generated") {
			return yellow
		}
		return white
	case "error":
		return red
	default:
		return ""
	}
}

func colorForStatus(value string) string {
	status, err := strconv.Atoi(strings.Trim(value, `"`))
	if err != nil {
		return white
	}
	if status >= 500 {
		return red
	}
	if status >= 400 {
		return yellow
	}
	if status >= 300 {
		return cyan
	}
	return green
}

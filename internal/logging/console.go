package logging

import (
	"bytes"
	"fmt"
	"io"
	"os"
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
	text := strings.TrimRight(line, "\r\n")
	newline := line[len(text):]
	if newline == "" {
		newline = "\n"
	}
	if !w.color {
		return formatConsoleLine(time.Now().Format("2006/01/02 15:04:05"), text, false) + newline
	}
	timestamp := dim + cyan + time.Now().Format("2006/01/02 15:04:05") + reset
	return formatConsoleLine(timestamp, text, true) + newline
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
	var builder strings.Builder
	for index := 0; index < len(line); {
		start, end, key, value, ok := nextLogField(line, index)
		if !ok {
			builder.WriteString(line[index:])
			break
		}
		builder.WriteString(line[index:start])
		match := line[start:end]
		color := colorForLogField(key, value)
		if color == "" {
			builder.WriteString(match)
		} else {
			builder.WriteString(dim)
			builder.WriteString(key)
			builder.WriteString("=")
			builder.WriteString(reset)
			builder.WriteString(color)
			builder.WriteString(value)
			builder.WriteString(reset)
		}
		index = end
	}
	return builder.String()
}

type logField struct {
	key   string
	value string
}

func formatConsoleLine(timestamp, line string, color bool) string {
	prefix, fields, ok := parseLogFields(line)
	if !ok {
		if color {
			line = colorizeLogFields(line)
		}
		return timestamp + " " + line
	}

	var builder strings.Builder
	builder.WriteString(timestamp)
	if prefix != "" {
		builder.WriteString(" ")
		builder.WriteString(prefix)
	}
	for _, field := range fields {
		builder.WriteString("\n  ")
		if color {
			builder.WriteString(colorizeLogField(field.key, field.value))
			continue
		}
		builder.WriteString(field.key)
		builder.WriteString("=")
		builder.WriteString(field.value)
	}
	return builder.String()
}

func parseLogFields(line string) (string, []logField, bool) {
	var fields []logField
	firstFieldStart := -1
	for index := 0; index < len(line); {
		start, end, key, value, ok := nextLogField(line, index)
		if !ok {
			break
		}
		if firstFieldStart == -1 {
			firstFieldStart = start
		}
		fields = append(fields, logField{key: key, value: value})
		index = end
	}
	if len(fields) == 0 {
		return "", nil, false
	}
	return strings.TrimSpace(line[:firstFieldStart]), fields, true
}

func colorizeLogField(key, value string) string {
	color := colorForLogField(key, value)
	if color == "" {
		return key + "=" + value
	}
	return dim + key + "=" + reset + color + value + reset
}

func nextLogField(line string, offset int) (int, int, string, string, bool) {
	for index := offset; index < len(line); index++ {
		if !isFieldKeyStart(line[index]) || (index > 0 && isFieldKeyChar(line[index-1])) {
			continue
		}
		keyEnd := index + 1
		for keyEnd < len(line) && isFieldKeyChar(line[keyEnd]) {
			keyEnd++
		}
		if keyEnd >= len(line) || line[keyEnd] != '=' {
			continue
		}
		valueStart := keyEnd + 1
		valueEnd := scanLogFieldValue(line, valueStart)
		if valueEnd == valueStart {
			continue
		}
		return index, valueEnd, line[index:keyEnd], line[valueStart:valueEnd], true
	}
	return 0, 0, "", "", false
}

func scanLogFieldValue(line string, start int) int {
	if start >= len(line) {
		return start
	}
	if line[start] != '"' {
		end := start
		for end < len(line) && !isLogFieldSeparator(line[end]) {
			end++
		}
		return end
	}
	escaped := false
	for end := start + 1; end < len(line); end++ {
		if escaped {
			escaped = false
			continue
		}
		if line[end] == '\\' {
			escaped = true
			continue
		}
		if line[end] == '"' {
			return end + 1
		}
	}
	return len(line)
}

func isLogFieldSeparator(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}

func isFieldKeyStart(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isFieldKeyChar(char byte) bool {
	return isFieldKeyStart(char) || (char >= '0' && char <= '9')
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
	case "bytes", "requestBytes", "responseBytes", "size", "items", "mediaItems", "comments", "activityItems":
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

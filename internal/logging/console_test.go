package logging

import (
	"strings"
	"testing"
)

func TestColorizeLogFieldsKeepsQuotedValuesWithSpacesTogether(t *testing.T) {
	line := `request path="/media/id" filename="My Video File.mp4" duration=4ms`

	colored := colorizeLogFields(line)

	if strings.Contains(colored, reset+" Video File.mp4") {
		t.Fatalf("expected quoted filename value to stay inside one color span, got %q", colored)
	}
	if !strings.Contains(colored, cyan+`"My Video File.mp4"`+reset) {
		t.Fatalf("expected full quoted filename to be colored, got %q", colored)
	}
}

func TestColorizeLogFieldsKeepsEscapedQuotesInsideQuotedValues(t *testing.T) {
	line := `request filename="My \"quoted\" Video.mp4" duration=4ms`

	colored := colorizeLogFields(line)

	if !strings.Contains(colored, cyan+`"My \"quoted\" Video.mp4"`+reset) {
		t.Fatalf("expected escaped quoted filename to be colored as one value, got %q", colored)
	}
}

func TestFormatConsoleLineSplitsLogFieldsAcrossLines(t *testing.T) {
	line := `request method=GET path="/media/id" query="" status=200 requestBytes=0 responseBytes=123 duration=4ms`

	formatted := formatConsoleLine("2026/05/31 12:00:00", line, false)

	want := strings.Join([]string{
		"2026/05/31 12:00:00 request",
		"  method=GET",
		`  path="/media/id"`,
		`  query=""`,
		"  status=200",
		"  requestBytes=0",
		"  responseBytes=123",
		"  duration=4ms",
	}, "\n")
	if formatted != want {
		t.Fatalf("expected multiline key-value log:\n%s\ngot:\n%s", want, formatted)
	}
}

func TestFormatConsoleLineKeepsPlainLogsSingleLine(t *testing.T) {
	line := "server started"

	formatted := formatConsoleLine("2026/05/31 12:00:00", line, false)

	if formatted != "2026/05/31 12:00:00 server started" {
		t.Fatalf("expected plain log to stay single-line, got %q", formatted)
	}
}

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

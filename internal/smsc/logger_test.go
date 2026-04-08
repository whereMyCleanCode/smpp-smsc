package smsc

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLoggerWithOptionsPrettyMode(t *testing.T) {
	var out bytes.Buffer

	lgr := NewLoggerWithOptions(outWriter(&out), InfoLevel, LoggerOptions{
		Pretty:     true,
		Color:      false,
		TimeFormat: "15:04:05",
	})

	zl, ok := lgr.(*zapLogger)
	if !ok {
		t.Fatalf("expected zerologLogger implementation")
	}
	if !zl.pretty {
		t.Fatalf("expected pretty mode")
	}
	if zl.color {
		t.Fatalf("expected color disabled")
	}

	lgr.Info().Str("k", "v").Msg("pretty mode")
	logLine := out.String()

	if strings.Contains(logLine, "\"level\":\"info\"") {
		t.Fatalf("pretty log should not be strict json: %s", logLine)
	}
	if !strings.Contains(logLine, "pretty mode") {
		t.Fatalf("missing message in output: %s", logLine)
	}
}

func TestNewLoggerWithOptionsJSONMode(t *testing.T) {
	var out bytes.Buffer

	lgr := NewLoggerWithOptions(outWriter(&out), InfoLevel, LoggerOptions{
		Pretty: false,
		Color:  false,
	})
	zl, ok := lgr.(*zapLogger)
	if !ok {
		t.Fatalf("expected zerologLogger implementation")
	}
	if zl.pretty {
		t.Fatalf("expected json mode")
	}

	lgr.Info().Str("component", "test").Msg("json mode")
	logLine := out.String()

	if !strings.Contains(logLine, "\"level\":\"info\"") {
		t.Fatalf("json output should contain level field: %s", logLine)
	}
	if !strings.Contains(logLine, "\"msg\":\"json mode\"") {
		t.Fatalf("json output should contain msg field: %s", logLine)
	}
}

func TestLoggerLevelWithColorDoesNotBreakFiltering(t *testing.T) {
	var out bytes.Buffer

	lgr := NewLoggerWithOptions(outWriter(&out), WarnLevel, LoggerOptions{
		Pretty: true,
		Color:  true,
	})

	lgr.Info().Msg("must not appear")
	lgr.Warn().Msg("must appear")

	logText := out.String()
	if strings.Contains(logText, "must not appear") {
		t.Fatalf("info message should be filtered out by warn level: %s", logText)
	}
	if !strings.Contains(logText, "must appear") {
		t.Fatalf("warn message should be present: %s", logText)
	}
}

func outWriter(buf *bytes.Buffer) *bytes.Buffer {
	return buf
}

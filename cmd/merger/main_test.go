package main

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestConfigureLogging(t *testing.T) {
	for _, tc := range []struct {
		value   string
		levels  []string
		invalid bool
	}{
		{"", []string{"INFO", "WARN", "ERROR"}, false},
		{"DEBUG", []string{"DEBUG", "INFO", "WARN", "ERROR"}, false},
		{"INFO", []string{"INFO", "WARN", "ERROR"}, false},
		{"WARN", []string{"WARN", "ERROR"}, false},
		{"ERROR", []string{"ERROR"}, false},
		{"invalid", []string{"INFO", "WARN", "ERROR"}, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			output, err := os.CreateTemp(t.TempDir(), "logs")
			if err != nil {
				t.Fatal(err)
			}
			previousStderr, previousLogger := os.Stderr, slog.Default()
			os.Stderr = output
			t.Cleanup(func() {
				os.Stderr = previousStderr
				slog.SetDefault(previousLogger)
				output.Close()
			})
			configureLogging(tc.value)
			slog.Debug("debug message")
			slog.Info("info message")
			slog.Warn("warn message")
			slog.Error("error message")
			data, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			logs := string(data)
			if strings.Contains(logs, "time=") || strings.Contains(logs, "request_id=") {
				t.Fatalf("unexpected timestamp or request ID: %s", logs)
			}
			for _, level := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
				want := false
				for _, enabled := range tc.levels {
					want = want || enabled == level
				}
				message := `msg="` + strings.ToLower(level) + ` message"`
				if got := strings.Contains(logs, "level="+level+" "+message); got != want {
					t.Errorf("level %s present = %v, want %v: %s", level, got, want, logs)
				}
			}
			if got := strings.Contains(logs, "invalid LOG_LEVEL; using INFO"); got != tc.invalid {
				t.Errorf("invalid level warning = %v, want %v: %s", got, tc.invalid, logs)
			}
			if strings.Contains(logs, "method=") || strings.Contains(logs, "path=") || strings.Contains(logs, "response_type=") {
				t.Fatalf("global logs contain request fields: %s", logs)
			}
		})
	}
}

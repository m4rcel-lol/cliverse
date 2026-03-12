package commands

import (
	"testing"
	"time"
)

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no tags", "hello world", "hello world"},
		{"simple tag", "<b>hello</b>", "hello"},
		{"nested tags", "<p><b>hello</b></p>", "hello"},
		{"self-closing", "hello<br/>world", "helloworld"},
		{"script tag", "<script>alert('xss')</script>", "alert('xss')"},
		{"empty", "", ""},
		{"only tags", "<p></p>", ""},
		{"attributes", `<a href="http://evil.com">click</a>`, "click"},
		{"angle brackets in text", "1 < 2 and 3 > 2", "1  2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTMLTags(tt.input)
			if got != tt.want {
				t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		dur  time.Duration
		want string
	}{
		{"zero", 0, "0m"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours and minutes", 2*time.Hour + 30*time.Minute, "2h 30m"},
		{"days", 25*time.Hour + 15*time.Minute, "1d 1h 15m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.dur)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

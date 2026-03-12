package ssh

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple command",
			input:    "help",
			expected: []string{"help"},
		},
		{
			name:     "command with args",
			input:    "post global hello",
			expected: []string{"post", "global", "hello"},
		},
		{
			name:     "quoted argument",
			input:    `post global "hello world"`,
			expected: []string{"post", "global", "hello world"},
		},
		{
			name:     "multiple spaces",
			input:    "post   global   hello",
			expected: []string{"post", "global", "hello"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: nil,
		},
		{
			name:     "quoted with spaces",
			input:    `admin create_user alice "ssh.mreow.org/m"`,
			expected: []string{"admin", "create_user", "alice", "ssh.mreow.org/m"},
		},
		{
			name:     "empty quotes",
			input:    `post global ""`,
			expected: []string{"post", "global"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommand(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("parseCommand(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.expected, len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseCommand(%q)[%d] = %q, want %q",
						tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

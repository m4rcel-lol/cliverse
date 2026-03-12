package commands

import "testing"

func TestValidUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"lowercase alpha", "alice", true},
		{"with numbers", "alice42", true},
		{"with underscore", "alice_bob", true},
		{"all numbers", "12345", true},
		{"single underscore", "_", true},
		{"mixed", "a1_b2", true},
		{"uppercase", "Alice", false},
		{"has dash", "alice-bob", false},
		{"has dot", "alice.bob", false},
		{"has space", "alice bob", false},
		{"has at sign", "@alice", false},
		{"has special", "alice!", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validUsername.MatchString(tt.input)
			if got != tt.valid {
				t.Errorf("validUsername(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

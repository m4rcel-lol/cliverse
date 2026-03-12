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

func TestSSHKeyName(t *testing.T) {
	tests := []struct {
		name        string
		keyStr      string
		fingerprint string
		want        string
	}{
		{
			name:        "ed25519 key",
			keyStr:      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@example",
			fingerprint: "SHA256:abcdef123456",
			want:        "ssh-ed25519 ef123456",
		},
		{
			name:        "short fingerprint",
			keyStr:      "ssh-rsa AAAA",
			fingerprint: "short",
			want:        "ssh-rsa",
		},
		{
			name:        "empty key string",
			keyStr:      "",
			fingerprint: "SHA256:abcdef123456",
			want:        "SHA256:abcdef123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sshKeyName(tt.keyStr, tt.fingerprint)
			if got != tt.want {
				t.Errorf("sshKeyName(%q, %q) = %q, want %q",
					tt.keyStr, tt.fingerprint, got, tt.want)
			}
		})
	}
}

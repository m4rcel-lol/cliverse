package activitypub

import "testing"

func TestParseAcctResource(t *testing.T) {
	tests := []struct {
		name        string
		resource    string
		localDomain string
		wantUser    string
		wantErr     bool
	}{
		{
			name:        "valid acct resource",
			resource:    "acct:alice@example.com",
			localDomain: "example.com",
			wantUser:    "alice",
		},
		{
			name:        "without acct prefix",
			resource:    "alice@example.com",
			localDomain: "example.com",
			wantUser:    "alice",
		},
		{
			name:        "domain mismatch",
			resource:    "acct:alice@other.com",
			localDomain: "example.com",
			wantErr:     true,
		},
		{
			name:        "missing @ sign",
			resource:    "acct:alice",
			localDomain: "example.com",
			wantErr:     true,
		},
		{
			name:        "empty resource",
			resource:    "",
			localDomain: "example.com",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAcctResource(tt.resource, tt.localDomain)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAcctResource(%q, %q) error = %v, wantErr %v",
					tt.resource, tt.localDomain, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantUser {
				t.Errorf("parseAcctResource(%q, %q) = %q, want %q",
					tt.resource, tt.localDomain, got, tt.wantUser)
			}
		})
	}
}

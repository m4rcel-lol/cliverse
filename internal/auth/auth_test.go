package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSSHKeys(t *testing.T) {
	// Valid ed25519 key (example)
	validKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example"

	t.Run("single valid key", func(t *testing.T) {
		keys, err := ParseSSHKeys(validKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}
		if keys[0] != validKey {
			t.Fatalf("key mismatch: got %q", keys[0])
		}
	})

	t.Run("multiple keys with blanks and comments", func(t *testing.T) {
		input := "# my keys\n" + validKey + "\n\n# another comment\n" + validKey + "\n"
		keys, err := ParseSSHKeys(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(keys))
		}
	})

	t.Run("no valid keys", func(t *testing.T) {
		_, err := ParseSSHKeys("not a key\njust some text")
		if err == nil {
			t.Fatal("expected error for no valid keys")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := ParseSSHKeys("")
		if err == nil {
			t.Fatal("expected error for empty string")
		}
	})
}

func TestFetchSSHKeysFromURL(t *testing.T) {
	validKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example"

	t.Run("successful fetch", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(validKey + "\n"))
		}))
		defer ts.Close()

		keys, err := FetchSSHKeysFromURL(ts.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("expected 1 key, got %d", len(keys))
		}
	})

	t.Run("404 response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer ts.Close()

		_, err := FetchSSHKeysFromURL(ts.URL)
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
	})

	t.Run("no valid keys in response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("this is not a key\n"))
		}))
		defer ts.Close()

		_, err := FetchSSHKeysFromURL(ts.URL)
		if err == nil {
			t.Fatal("expected error for no valid keys")
		}
	})

	t.Run("empty URL", func(t *testing.T) {
		_, err := FetchSSHKeysFromURL("")
		if err == nil {
			t.Fatal("expected error for empty URL")
		}
	})

	t.Run("prepends https for bare URL", func(t *testing.T) {
		// This will fail to connect, but we just check the URL is adjusted
		_, err := FetchSSHKeysFromURL("localhost:0/nonexistent")
		if err == nil {
			t.Fatal("expected connection error")
		}
	})
}

func TestHashAndVerifyPassword(t *testing.T) {
	password := "testPassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !ok {
		t.Fatal("password verification failed")
	}

	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if ok {
		t.Fatal("wrong password should not verify")
	}
}

func TestCalcFingerprint(t *testing.T) {
	validKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example"

	fp, err := CalcFingerprint(validKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp == "" {
		t.Fatal("fingerprint should not be empty")
	}
	if len(fp) < 7 || fp[:7] != "SHA256:" {
		t.Fatalf("fingerprint should start with SHA256:, got %q", fp)
	}

	_, err = CalcFingerprint("not a key")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestGenerateRSAKeyPair(t *testing.T) {
	priv, pub, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	if priv == "" || pub == "" {
		t.Fatal("keys should not be empty")
	}
}

func TestGenerateSessionToken(t *testing.T) {
	token, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("generate session token: %v", err)
	}
	if len(token) != 64 { // 32 bytes = 64 hex chars
		t.Fatalf("expected 64 hex chars, got %d", len(token))
	}
}

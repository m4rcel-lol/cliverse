package activitypub

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"strings"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	// Generate a fresh RSA key pair for the test.
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privDER := x509.MarshalPKCS1PrivateKey(privKey)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER}))

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	req, _ := http.NewRequest("POST", "https://example.com/users/alice/inbox", strings.NewReader(`{}`))
	req.Header.Set("Host", "example.com")

	// Sign the request.
	if err := Sign(req, "https://remote.example/users/bob#main-key", privPEM); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Verify that the Signature header was set.
	sigHeader := req.Header.Get("Signature")
	if sigHeader == "" {
		t.Fatal("expected Signature header to be set")
	}
	if !strings.Contains(sigHeader, "rsa-sha256") {
		t.Errorf("expected rsa-sha256 in Signature header, got %q", sigHeader)
	}

	// Verify the signature.
	if err := Verify(req, pubPEM); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyRejectsTamperedRequest(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privDER := x509.MarshalPKCS1PrivateKey(privKey)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER}))
	pubDER, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	req, _ := http.NewRequest("POST", "https://example.com/users/alice/inbox", nil)
	req.Header.Set("Host", "example.com")
	_ = Sign(req, "key-id", privPEM)

	// Tamper with the Host header after signing.
	req.Header.Set("Host", "evil.example.com")

	if err := Verify(req, pubPEM); err == nil {
		t.Fatal("expected Verify to fail after tampering")
	}
}

func TestVerifyMissingSignatureHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://example.com/users/alice", nil)
	err := Verify(req, "irrelevant")
	if err == nil {
		t.Fatal("expected error for missing Signature header")
	}
}

func TestAddDigest(t *testing.T) {
	body := []byte(`{"type":"Follow"}`)
	req, _ := http.NewRequest("POST", "https://example.com/inbox", nil)
	AddDigest(req, body)

	digest := req.Header.Get("Digest")
	if digest == "" {
		t.Fatal("expected Digest header to be set")
	}
	if !strings.HasPrefix(digest, "SHA-256=") {
		t.Errorf("expected SHA-256= prefix, got %q", digest)
	}

	// Verify the digest is correct.
	h := sha256.Sum256(body)
	expected := "SHA-256=" + base64.StdEncoding.EncodeToString(h[:])
	if digest != expected {
		t.Errorf("digest mismatch: got %q, want %q", digest, expected)
	}
}

func TestParseSignatureHeader(t *testing.T) {
	header := `keyId="https://example.com/users/bob#main-key",algorithm="rsa-sha256",headers="(request-target) host date",signature="abc123"`
	params := parseSignatureHeader(header)

	if params["keyId"] != "https://example.com/users/bob#main-key" {
		t.Errorf("unexpected keyId: %q", params["keyId"])
	}
	if params["algorithm"] != "rsa-sha256" {
		t.Errorf("unexpected algorithm: %q", params["algorithm"])
	}
	if params["signature"] != "abc123" {
		t.Errorf("unexpected signature: %q", params["signature"])
	}
}

func TestDefaultContext(t *testing.T) {
	ctx := DefaultContext()
	if len(ctx) != 2 {
		t.Fatalf("expected 2 context entries, got %d", len(ctx))
	}
	if ctx[0] != ASContext {
		t.Errorf("first context entry should be %q, got %q", ASContext, ctx[0])
	}
	if ctx[1] != W3IDSec {
		t.Errorf("second context entry should be %q, got %q", W3IDSec, ctx[1])
	}
}

// Ensure SignPKCS1v15 verifies against known constants.
func TestSignatureRoundTrip(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	message := "test message to sign"
	h := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(&privKey.PublicKey, crypto.SHA256, h[:], sig); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

package activitypub

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AddDigest computes the SHA-256 digest of body and sets the Digest header on r.
func AddDigest(r *http.Request, body []byte) {
	sum := sha256.Sum256(body)
	r.Header.Set("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(sum[:]))
}

// Sign signs the HTTP request using RSA-SHA256 HTTP Signatures. It signs the
// pseudo-header (request-target), host, and date. If the request has a body
// (non-nil Body or a non-empty Content-Length), the digest header must already
// be present and is included in the signature.
func Sign(r *http.Request, keyID string, privateKeyPEM string) error {
	// Parse the private key.
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return fmt.Errorf("activitypub/sign: failed to decode PEM block")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format as a fallback.
		key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return fmt.Errorf("activitypub/sign: parse private key: %w", err)
		}
		var ok bool
		privKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("activitypub/sign: private key is not RSA")
		}
	}

	// Ensure required headers are set.
	if r.Header.Get("Date") == "" {
		r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}
	if r.Header.Get("Host") == "" {
		r.Header.Set("Host", r.URL.Host)
	}

	// Build the list of headers to sign.
	headers := []string{"(request-target)", "host", "date"}
	if r.Header.Get("Digest") != "" {
		headers = append(headers, "digest")
	}

	// Build the signing string.
	var sb strings.Builder
	for i, h := range headers {
		if i > 0 {
			sb.WriteString("\n")
		}
		switch h {
		case "(request-target)":
			target := strings.ToLower(r.Method) + " " + r.URL.RequestURI()
			sb.WriteString("(request-target): " + target)
		case "host":
			host := r.Header.Get("Host")
			if host == "" {
				host = r.URL.Host
			}
			sb.WriteString("host: " + host)
		default:
			sb.WriteString(h + ": " + r.Header.Get(http.CanonicalHeaderKey(h)))
		}
	}

	// Sign with RSA-SHA256.
	hash := sha256.Sum256([]byte(sb.String()))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		return fmt.Errorf("activitypub/sign: rsa sign: %w", err)
	}

	// Build the Signature header value.
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	headerList := strings.Join(headers, " ")
	sigHeader := fmt.Sprintf(
		`keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
		keyID, headerList, sigB64,
	)
	r.Header.Set("Signature", sigHeader)
	return nil
}

// Verify verifies the HTTP Signature on an incoming request using the provided
// RSA public key in PEM format. It returns an error if the signature is missing,
// malformed, or cryptographically invalid.
func Verify(r *http.Request, publicKeyPEM string) error {
	sigHeader := r.Header.Get("Signature")
	if sigHeader == "" {
		return fmt.Errorf("activitypub/verify: missing Signature header")
	}

	// Parse the Signature header into key=value pairs.
	params := parseSignatureHeader(sigHeader)

	algorithm := params["algorithm"]
	if algorithm != "" && algorithm != "rsa-sha256" {
		return fmt.Errorf("activitypub/verify: unsupported algorithm: %s", algorithm)
	}

	headersParam := params["headers"]
	if headersParam == "" {
		headersParam = "date"
	}
	headers := strings.Fields(headersParam)

	sigB64 := params["signature"]
	if sigB64 == "" {
		return fmt.Errorf("activitypub/verify: missing signature value")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("activitypub/verify: decode signature: %w", err)
	}

	// Reconstruct the signing string.
	var sb strings.Builder
	for i, h := range headers {
		if i > 0 {
			sb.WriteString("\n")
		}
		switch h {
		case "(request-target)":
			target := strings.ToLower(r.Method) + " " + r.URL.RequestURI()
			sb.WriteString("(request-target): " + target)
		case "host":
			host := r.Header.Get("Host")
			if host == "" {
				host = r.Host
			}
			sb.WriteString("host: " + host)
		default:
			sb.WriteString(h + ": " + r.Header.Get(http.CanonicalHeaderKey(h)))
		}
	}

	// Parse the public key.
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return fmt.Errorf("activitypub/verify: failed to decode public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("activitypub/verify: parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("activitypub/verify: public key is not RSA")
	}

	hash := sha256.Sum256([]byte(sb.String()))
	if err := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hash[:], sigBytes); err != nil {
		return fmt.Errorf("activitypub/verify: invalid signature: %w", err)
	}
	return nil
}

// parseSignatureHeader parses a Signature header value into a map of key=value pairs.
func parseSignatureHeader(header string) map[string]string {
	params := make(map[string]string)
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(part[:idx])
		val := strings.TrimSpace(part[idx+1:])
		val = strings.Trim(val, `"`)
		params[key] = val
	}
	return params
}

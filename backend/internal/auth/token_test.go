package auth

import (
	"encoding/base64"
	"regexp"
	"testing"
)

func TestGenerateToken_EntropyAndHash(t *testing.T) {
	plain, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	// Plaintext decodes to the 256-bit (32-byte) entropy floor.
	raw, err := base64.RawURLEncoding.DecodeString(plain)
	if err != nil {
		t.Fatalf("plaintext is not base64url: %v", err)
	}
	if len(raw) != setPasswordTokenBytes {
		t.Errorf("token entropy = %d bytes, want %d", len(raw), setPasswordTokenBytes)
	}
	// Hash is the SHA-256 hex of the plaintext, and the stored value is not the
	// plaintext itself (so a DB leak yields nothing usable).
	if hash != HashToken(plain) {
		t.Error("returned hash does not match HashToken(plaintext)")
	}
	if hash == plain {
		t.Error("stored hash equals plaintext — token kept in the clear")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash) {
		t.Errorf("hash is not 64-char lowercase hex: %q", hash)
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		plain, hash, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		if seen[plain] {
			t.Fatal("GenerateToken produced a duplicate plaintext")
		}
		if seen[hash] {
			t.Fatal("GenerateToken produced a duplicate hash")
		}
		seen[plain] = true
		seen[hash] = true
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	first := HashToken("the-same-token")
	second := HashToken("the-same-token")
	if first != second {
		t.Error("HashToken is not deterministic")
	}
	if HashToken("token-a") == HashToken("token-b") {
		t.Error("HashToken collided on distinct inputs")
	}
}

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// Shared set-password-token primitive (ADR-0039, #281). One mechanism backs the
// invite-link join here and the later reset/reactivation slices: a single-use,
// short-TTL token with ≥256-bit entropy whose plaintext lives only in the
// emailed link and is *never* stored or logged. At rest we keep a SHA-256 hash,
// so a database leak yields nothing usable.
//
// The token is high-entropy random, not a password, so a fast hash is the
// correct choice — there is no low-entropy secret to slow down brute-forcing of
// (Argon2id would be miscost here). Lookup hashes the presented token and
// queries by hash; the column is unique, so the hash is the key. Single-use and
// TTL are enforced by each consumer's atomic UPDATE … WHERE used_at IS NULL AND
// expires_at > now(), not by this primitive.

// setPasswordTokenBytes is 32 random bytes = 256 bits of entropy, the ADR floor.
const setPasswordTokenBytes = 32

// RelayTokenTTL is the validity window for a set-password link that is relayed
// out-of-band by a human — the founder-assisted reactivation link (#283) and the
// operator-CLI reset link (#284). It is generous relative to the 1h emailed-reset
// TTL because the operator/founder must hand the link over by hand (copy-paste,
// chat, in person) before the member follows it; the token's single-use +
// hashed-at-rest discipline (not a short clock) is what bounds its blast radius.
const RelayTokenTTL = 72 * time.Hour

// GenerateToken returns a fresh random set-password token: the URL-safe plaintext
// to put in the link, and the hash to store. The plaintext is the only copy that
// can be presented; once returned it is the caller's responsibility never to
// persist or log it.
func GenerateToken() (plaintext, hash string, err error) {
	b := make([]byte, setPasswordTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(b)
	return plaintext, HashToken(plaintext), nil
}

// HashToken maps a plaintext token to its at-rest representation: lowercase hex
// of the SHA-256 digest. Deterministic (no per-hash salt) so a presented token
// can be looked up by hashing then querying — the token's own entropy, not a
// salt, is what makes the stored value useless to an attacker.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

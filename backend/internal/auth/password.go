package auth

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (ADR-0039). Tuned for the canonical self-host target — a
// single-board computer (Raspberry Pi), not a server — so memory cost is the
// knob that matters and is kept at the OWASP-recommended floor rather than
// pushed higher. m=19 MiB, t=2, p=1 is OWASP's first listed Argon2id option and
// hashes in well under a second on a Pi 4. The values are encoded into every
// PHC string we store, so they can be retuned later with no schema change and
// old hashes keep verifying against their own recorded cost.
const (
	argonMemoryKiB  = 19 * 1024 // 19456 KiB = 19 MiB
	argonTime       = 2
	argonThreads    = 1
	argonKeyLen     = 32
	argonSaltLen    = 16
	argonAlgo       = "argon2id"
	argonPHCVersion = argon2.Version // 19 (0x13)
)

// minPasswordLen is the only length rule (ADR-0039): a floor, no composition
// rules and no forced rotation — modern guidance (NIST 800-63B) favours length
// and a breach check over character-class theatre.
const minPasswordLen = 10

// errWeakPassword variants are surfaced to the caller as a single generic
// VALIDATION code; the specific reason rides the args so the SPA can show a
// helpful message without the backend leaking a policy oracle beyond the form.
var (
	errPasswordTooShort = errors.New("password below minimum length")
	errPasswordBreached = errors.New("password is among commonly-breached passwords")
)

//go:embed common_passwords.txt
var commonPasswordsRaw string

// commonPasswords is the breached/common reject set (lower-cased). Loaded once
// at package init from the embedded list — a small curated top-N rather than a
// full HIBP corpus: it blocks the passwords actually tried first in online
// guessing without shipping a multi-megabyte file onto an SBC.
var commonPasswords = func() map[string]struct{} {
	set := make(map[string]struct{})
	for _, line := range strings.Split(commonPasswordsRaw, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			set[strings.ToLower(p)] = struct{}{}
		}
	}
	return set
}()

// ValidatePasswordPolicy enforces the password floor (ADR-0039): a minimum
// length and a reject of commonly-breached passwords. Returns a typed error so
// the handler maps it to a VALIDATION envelope with a reason arg. Length is
// counted in runes, not bytes, so a short multi-byte password is not over-counted.
func ValidatePasswordPolicy(password string) error {
	if utf8.RuneCountInString(password) < minPasswordLen {
		return errPasswordTooShort
	}
	if _, banned := commonPasswords[strings.ToLower(password)]; banned {
		return errPasswordBreached
	}
	return nil
}

// HashPassword returns an Argon2id PHC string ($argon2id$v=19$m=...,t=...,p=...$salt$hash)
// with a fresh random salt. The salt and cost parameters travel inside the
// returned string, so verification needs nothing else stored. Caller is expected
// to have already run ValidatePasswordPolicy.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonAlgo, argonPHCVersion, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword reports whether password matches the stored PHC hash. It parses
// the cost parameters out of the stored string (so a hash made under different
// params still verifies), recomputes, and compares in constant time. A malformed
// stored string returns false with an error — never a panic and never a match.
func VerifyPassword(password, phc string) (bool, error) {
	algo, version, mem, t, p, salt, want, err := parsePHC(phc)
	if err != nil {
		return false, err
	}
	if algo != argonAlgo {
		return false, fmt.Errorf("unsupported algorithm %q", algo)
	}
	if version != argonPHCVersion {
		return false, fmt.Errorf("unsupported argon2 version %d", version)
	}
	got := argon2.IDKey([]byte(password), salt, t, mem, uint8(p), uint32(len(want)))
	// subtle.ConstantTimeCompare returns 1 on equal; the length guard it implies
	// is fine here — both are argonKeyLen-derived for our own hashes.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// parsePHC splits a $argon2id$v=..$m=..,t=..,p=..$salt$hash string into its
// parts. Strict: any structural deviation is an error, so a corrupt or
// foreign-format value can never be mistaken for a verifiable hash.
func parsePHC(phc string) (algo string, version, mem, t, p uint32, salt, hash []byte, err error) {
	parts := strings.Split(phc, "$")
	// ["", algo, "v=N", "m=..,t=..,p=..", salt, hash]
	if len(parts) != 6 || parts[0] != "" {
		err = errors.New("malformed PHC string")
		return
	}
	algo = parts[1]
	if _, e := fmt.Sscanf(parts[2], "v=%d", &version); e != nil {
		err = fmt.Errorf("parse version: %w", e)
		return
	}
	if _, e := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); e != nil {
		err = fmt.Errorf("parse params: %w", e)
		return
	}
	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		err = fmt.Errorf("decode salt: %w", err)
		return
	}
	if hash, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		err = fmt.Errorf("decode hash: %w", err)
		return
	}
	return
}

// dummyArgonHash is a precomputed valid PHC hash of a throwaway value, used to
// equalise login response timing when no user/credential exists: the login
// handler runs VerifyPassword against this constant so a missing account costs
// the same Argon2id work as a present one (no user enumeration via timing).
//
//nolint:gochecknoglobals // intentional package-level constant work factor
var dummyArgonHash, _ = HashPassword("dummy-password-for-constant-time-login")

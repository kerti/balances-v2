package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	phc, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(phc, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("PHC string has unexpected prefix: %q", phc)
	}
	ok, err := VerifyPassword("correct horse battery staple", phc)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword: correct password did not match")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	phc, err := HashPassword("the right password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("the wrong password", phc)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("VerifyPassword: wrong password matched")
	}
}

func TestHashPassword_SaltedPerHash(t *testing.T) {
	a, _ := HashPassword("same-password-1234")
	b, _ := HashPassword("same-password-1234")
	if a == b {
		t.Error("two hashes of the same password are identical — salt is not random")
	}
}

func TestVerifyPassword_MalformedNeverMatches(t *testing.T) {
	for _, bad := range []string{
		"", "not-a-phc", "$argon2id$bad", "$bcrypt$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=99$m=19456,t=2,p=1$c2FsdA$aGFzaA",
	} {
		ok, err := VerifyPassword("whatever", bad)
		if ok {
			t.Errorf("VerifyPassword(%q) matched a malformed hash", bad)
		}
		if err == nil {
			t.Errorf("VerifyPassword(%q): expected an error", bad)
		}
	}
}

// covers: INV-AUTH-17
func TestValidatePasswordPolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"too short", "short", errPasswordTooShort},
		{"exactly nine", "123456789", errPasswordTooShort},
		{"breached common", "password123", errPasswordBreached},
		{"breached mixed case", "PassWord123", errPasswordBreached},
		{"good long unique", "a-perfectly-fine-passphrase", nil},
		{"ten chars ok", "abcdefghij", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePasswordPolicy(tc.password)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("ValidatePasswordPolicy(%q) = %v, want %v", tc.password, err, tc.wantErr)
			}
		})
	}
}

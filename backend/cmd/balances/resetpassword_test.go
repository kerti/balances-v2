package main

import "testing"

// TestParseResetPasswordArgs pins the CLI argument contract: exactly one
// non-empty <email> positional, anything else is a usage error. (The mint
// behaviour itself is covered against a real database in package auth.)
func TestParseResetPasswordArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"one email", []string{"member@example.com"}, "member@example.com", false},
		{"no args", nil, "", true},
		{"empty arg", []string{""}, "", true},
		{"too many", []string{"a@example.com", "b@example.com"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResetPasswordArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("email = %q, want %q", got, tt.want)
			}
		})
	}
}

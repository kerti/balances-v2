package main

import "testing"

// TestIsScannedTestFile pins the set of test-file kinds the matrix reads for
// `covers:` annotations: Go unit tests, Playwright specs, and vitest
// component/unit tests. The vitest suffixes (.test.ts / .test.tsx) are the
// reason this is worth a guard — they're new, and a regression would silently
// stop crediting every frontend annotation without changing any count.
func TestIsScannedTestFile(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"foo_test.go", true},
		{"login.spec.ts", true},
		{"costBasis.test.ts", true},
		{"Button.test.tsx", true},
		{"main.go", false},
		{"costBasis.ts", false},
		{"helpers.tsx", false},
		{"notes.test.js", false},
		{"README.md", false},
	}
	for _, c := range cases {
		if got := isScannedTestFile(c.name); got != c.want {
			t.Errorf("isScannedTestFile(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

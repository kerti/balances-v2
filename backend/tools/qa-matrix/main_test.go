package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

// TestTierForNextTest is the heart of the per-PR gate: a `covers:` annotation in
// a Playwright spec only counts toward the gate if the test it sits above is
// @smoke-tagged (those run per-PR; the rest run nightly, #70). A regression here
// would silently mis-tier E2E coverage and let -strict credit a test that didn't
// run in the PR — exactly the failure the tiering exists to prevent.
func TestTierForNextTest(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		start int
		want  tier
	}{
		{
			name:  "smoke tag on the test line (the common shape)",
			lines: []string{"// (annotation line — neutralised so the live scan ignores this fixture)", "test('does a thing', { tag: '@smoke' }, async ({ page }) => {"},
			start: 1,
			want:  tierPerPR,
		},
		{
			name:  "no tag, async on the test line",
			lines: []string{"// (annotation line — neutralised so the live scan ignores this fixture)", "test('does a thing', async ({ page }) => {"},
			start: 1,
			want:  tierNightly,
		},
		{
			name: "smoke tag wrapped onto a later signature line",
			lines: []string{
				"// (annotation line — neutralised so the live scan ignores this fixture)",
				"test('a very long name that wrapped',",
				"  { tag: '@smoke' },",
				"  async ({ page }) => {",
			},
			start: 1,
			want:  tierPerPR,
		},
		{
			name:  "no following test() at all is conservative (nightly)",
			lines: []string{"// (annotation line — neutralised so the live scan ignores this fixture)", "const noTestHere = 1"},
			start: 1,
			want:  tierNightly,
		},
		{
			name: "skips intervening non-test lines to reach the test()",
			lines: []string{
				"// (annotation line — neutralised so the live scan ignores this fixture)",
				"",
				"// a comment about the test",
				"test('thing', { tag: '@smoke' }, async () => {",
			},
			start: 1,
			want:  tierPerPR,
		},
	}
	for _, c := range cases {
		if got := tierForNextTest(c.lines, c.start); got != c.want {
			t.Errorf("%s: tierForNextTest = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestCoversInFileTiered wires file kind to tier end-to-end: a Go file's
// annotations are all per-PR; a spec's annotation inherits the tier of the
// test() below it. This is the integration point the gate depends on — the
// "<x>.spec.ts is special" branch — so it earns a real (temp-file) test.
func TestCoversInFileTiered(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "x_test.go")
	if err := os.WriteFile(goFile, []byte("// covers: INV-GO-01\nfunc TestX(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(dir, "x.spec.ts")
	specBody := "// covers: INV-SMOKE-01\n" +
		"test('a', { tag: '@smoke' }, async () => {})\n" +
		"// covers: INV-NIGHTLY-01\n" +
		"test('b', async () => {})\n"
	if err := os.WriteFile(spec, []byte(specBody), 0o644); err != nil {
		t.Fatal(err)
	}

	want := map[string]map[string]tier{
		goFile: {"INV-GO-01": tierPerPR},
		spec:   {"INV-SMOKE-01": tierPerPR, "INV-NIGHTLY-01": tierNightly},
	}
	for path, expect := range want {
		anns, err := coversInFileTiered(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		got := map[string]tier{}
		for _, a := range anns {
			got[a.id] = a.tier
		}
		for id, tr := range expect {
			if got[id] != tr {
				t.Errorf("%s: %s tier = %v, want %v", filepath.Base(path), id, got[id], tr)
			}
		}
		if len(got) != len(expect) {
			t.Errorf("%s: got %d annotations, want %d", filepath.Base(path), len(got), len(expect))
		}
	}
}

// TestAnyPerPR guards the gate's accept/reject decision: an invariant counts as
// per-PR-covered iff at least one covering location runs in the gate.
func TestAnyPerPR(t *testing.T) {
	cases := []struct {
		name string
		locs []location
		want bool
	}{
		{"empty is not per-PR", nil, false},
		{"only nightly is not per-PR", []location{{"e2e/x.spec.ts", tierNightly}}, false},
		{"a per-PR location counts", []location{{"x_test.go", tierPerPR}}, true},
		{"mixed counts (per-PR present)", []location{{"e2e/x.spec.ts", tierNightly}, {"x_test.go", tierPerPR}}, true},
	}
	for _, c := range cases {
		if got := anyPerPR(c.locs); got != c.want {
			t.Errorf("%s: anyPerPR = %v, want %v", c.name, got, c.want)
		}
	}
}

// Command qa-matrix joins the hand-authored QA invariant catalog (the per-zone
// files under docs/qa/invariants/) against `// covers: INV-...` annotations
// scattered across the Go + TypeScript test suites, regenerates the per-zone
// coverage files under docs/qa/coverage/ plus their index, and reports any
// invariant that no test claims to verify.
//
// The catalog is the source of truth for *what must hold*; the coverage files
// are computed here so they can never lie. Advisory by default (exit 0); -strict
// turns an uncovered invariant into a non-zero exit (the CI gate).
//
// Coverage is *tiered* by whether the covering test runs in the per-PR gate.
// Go (`_test.go`) and vitest (`.test.ts`/`.test.tsx`) always run per-PR (via
// `make check`/`frontend-checks`). Playwright specs are split (#70): only
// `@smoke`-tagged tests run per-PR; the rest run nightly. So an invariant
// covered *only* by a non-smoke spec is verified nightly, not in the gate —
// `-strict` treats that as a gap (a "nightly-only" finding), the same as an
// uncovered one, so the per-PR gate never credits coverage that didn't run in
// the PR. See docs/qa/how-it-works.md.
//
//	cd backend && go run ./tools/qa-matrix          # regenerate + report
//	cd backend && go run ./tools/qa-matrix -strict  # also fail on a gap (uncovered or nightly-only)
//	cd backend && go run ./tools/qa-matrix -gaps    # also list within-zone unannotated tests
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// A catalog row: | INV-ZONE-NN | statement | source | severity |
	catalogRow = regexp.MustCompile(`^\|\s*(INV-[A-Z0-9-]+)\s*\|\s*(.*?)\s*\|`)
	// A coverage annotation: `// covers: INV-A, INV-B` (Go or TS, same token).
	coversLine = regexp.MustCompile(`covers:\s*(INV-[A-Z0-9-]+(?:\s*,\s*INV-[A-Z0-9-]+)*)`)
	invID      = regexp.MustCompile(`INV-[A-Z0-9-]+`)
	// A zone catalog filename: NN-slug.md (the numeric prefix is the order).
	zoneFile = regexp.MustCompile(`^(\d+)-(.+)\.md$`)
	// The zone's title line: `# Zone: TENANCY`.
	zoneTitle = regexp.MustCompile(`^#\s+Zone:\s*(.+?)\s*$`)
)

// skipDirs are never walked — generated output, deps, and build artifacts.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	"vendor": true, "coverage": true, "playwright-report": true,
	"test-results": true,
}

// selfPkgRel is the matrix tool's own package, relative to repo root. Its
// *_test.go fixtures carry throwaway `// covers: INV-...` lines (inline string
// literals and temp files), so scanning it would surface those fixture IDs as
// spurious orphans (issue #234). Skip the whole package.
var selfPkgRel = filepath.Join("backend", "tools", "qa-matrix")

// isSelfPkg reports whether path is the matrix tool's own package directory.
func isSelfPkg(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == selfPkgRel
}

// isScannedTestFile reports whether a file is a test file the matrix reads for
// `covers:` annotations. Go unit tests (`_test.go`), Playwright specs
// (`.spec.ts`, run in CI per ADR-0024/#70), and vitest component/unit tests
// (`.test.ts` / `.test.tsx`, run every PR via `make check`) all qualify — the
// `covers:` token is language-agnostic by design.
func isScannedTestFile(name string) bool {
	for _, suffix := range []string{"_test.go", ".spec.ts", ".test.ts", ".test.tsx"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// tier records whether a covering test runs in the per-PR gate or only nightly.
// perPR < nightly so that when the same file yields both, perPR wins (the test
// that does run in the gate is the one that counts).
type tier int

const (
	tierPerPR   tier = iota // Go, vitest, or @smoke-tagged Playwright — runs in the per-PR gate
	tierNightly             // non-smoke Playwright — runs only in the nightly full suite
)

// location is one covering test file plus the tier it runs in.
type location struct {
	file string
	tier tier
}

// annotation is one `covers:` reference resolved to its tier within a file.
type annotation struct {
	id   string
	tier tier
}

type invariant struct {
	id, statement string
}

// zone is one catalog file: its output basename, display title, and the
// invariant IDs it defines, in file order.
type zone struct {
	file  string // basename, e.g. "01-tenancy.md" — mirrored into coverage/
	title string // display title, e.g. "TENANCY"
	ids   []string
}

func main() {
	root := flag.String("root", "", "repo root (default: nearest ancestor with a .git)")
	strictFlag := flag.Bool("strict", false, "exit non-zero if any catalogued invariant is uncovered (the CI gate)")
	reportFlag := flag.Bool("report", false, "print the summary only; don't rewrite the coverage files (used by `make check`)")
	gapsFlag := flag.Bool("gaps", false, "also list test files that carry no `covers:` annotation but sit in a directory where another test does (within-zone stragglers)")
	flag.Parse()

	r, err := resolveRoot(*root)
	if err != nil {
		fail(err)
	}

	catalogDir := filepath.Join(r, "docs", "qa", "invariants")
	invs, zones, order, err := parseCatalog(catalogDir)
	if err != nil {
		fail(fmt.Errorf("read catalog %s: %w", catalogDir, err))
	}

	coverage, err := scanCoverage(r)
	if err != nil {
		fail(fmt.Errorf("scan tests: %w", err))
	}

	// Orphans: a test references an ID the catalog never defines (typo or a
	// row that was deleted out from under its tests).
	var orphans []string
	for id := range coverage {
		if _, ok := invs[id]; !ok {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)

	// uncovered: no test annotates it at all. nightlyOnly: annotated, but every
	// covering test runs only nightly (non-smoke Playwright) — so the per-PR gate
	// would credit coverage that didn't run in the PR. Both are gaps for -strict.
	var uncovered, nightlyOnly []string
	for _, id := range order {
		locs := coverage[id]
		switch {
		case len(locs) == 0:
			uncovered = append(uncovered, id)
		case !anyPerPR(locs):
			nightlyOnly = append(nightlyOnly, id)
		}
	}

	outDir := filepath.Join(r, "docs", "qa", "coverage")
	rel, _ := filepath.Rel(r, outDir)
	rel = filepath.ToSlash(rel) + "/"
	if *reportFlag {
		rel = "(report-only, not written)"
	} else if err := writeCoverage(outDir, invs, zones, order, coverage, uncovered, nightlyOnly, orphans); err != nil {
		fail(fmt.Errorf("write %s: %w", outDir, err))
	}

	// Console summary. The headline tracks per-PR coverage — the number the gate
	// enforces — and breaks out nightly-only as a parenthetical so the "covered
	// somewhere" total is still visible.
	perPR := len(order) - len(uncovered) - len(nightlyOnly)
	fmt.Printf("qa-matrix: %d/%d invariants covered per-PR (%d nightly-only, %d uncovered) → %s\n",
		perPR, len(order), len(nightlyOnly), len(uncovered), rel)
	for _, id := range uncovered {
		fmt.Printf("  UNCOVERED    %s — %s\n", id, invs[id].statement)
	}
	for _, id := range nightlyOnly {
		fmt.Printf("  NIGHTLY-ONLY %s — covered only by a non-smoke spec; runs nightly, not in this gate\n", id)
	}
	for _, id := range orphans {
		fmt.Printf("  ORPHAN       %s — annotated by a test but absent from the catalog\n", id)
	}

	if *gapsFlag {
		gaps, err := scanGaps(r)
		if err != nil {
			fail(fmt.Errorf("scan gaps: %w", err))
		}
		fmt.Printf("\n%d within-zone unannotated test file(s) — a directory already verifies a\n", len(gaps))
		fmt.Println("catalogued invariant, so these are the likeliest to be guarding one without saying so:")
		for _, f := range gaps {
			fmt.Printf("  GAP       %s\n", f)
		}
	}

	if *strictFlag && (len(uncovered) > 0 || len(nightlyOnly) > 0) {
		os.Exit(1)
	}
}

// anyPerPR reports whether at least one covering location runs in the per-PR
// gate (Go, vitest, or an @smoke Playwright spec). An invariant covered only by
// nightly locations is not credited by the per-PR -strict gate.
func anyPerPR(locs []location) bool {
	for _, l := range locs {
		if l.tier == tierPerPR {
			return true
		}
	}
	return false
}

// scanGaps returns the test files that carry no `covers:` annotation but live
// in a directory where at least one sibling test file does. Test files in
// directories with zero annotations are excluded on purpose: an unannotated
// zone is an expected blank (a whole area not yet catalogued), not a straggler —
// listing them would drown the signal. This narrows "what isn't in the matrix"
// down to the files most likely to be silently verifying a catalogued invariant.
func scanGaps(root string) ([]string, error) {
	type dirState struct {
		unannotated []string
		annotated   int
	}
	dirs := map[string]*dirState{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || isSelfPkg(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !isScannedTestFile(name) {
			return nil
		}
		ids, err := coversInFile(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		st := dirs[dir]
		if st == nil {
			st = &dirState{}
			dirs[dir] = st
		}
		if len(ids) > 0 {
			st.annotated++
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		st.unannotated = append(st.unannotated, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}

	var gaps []string
	for _, st := range dirs {
		if st.annotated == 0 {
			continue
		}
		gaps = append(gaps, st.unannotated...)
	}
	sort.Strings(gaps)
	return gaps, nil
}

// resolveRoot returns the explicit root, else walks up from the cwd to the
// nearest directory containing a .git entry.
func resolveRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .git found above %s; pass -root", dir)
		}
		dir = parent
	}
}

// parseCatalog reads every NN-slug.md zone file under dir (in numeric-prefix
// order), returning the invariants by ID, the zones in display order, and the
// flat ID order (so the generated matrix mirrors the catalog).
func parseCatalog(dir string) (map[string]invariant, []zone, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !zoneFile.MatchString(e.Name()) {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files) // numeric prefix => risk order

	invs := map[string]invariant{}
	var zones []zone
	var order []string
	for _, name := range files {
		z, fileInvs, fileOrder, err := parseZoneFile(filepath.Join(dir, name))
		if err != nil {
			return nil, nil, nil, err
		}
		for id, inv := range fileInvs {
			if _, dup := invs[id]; dup {
				return nil, nil, nil, fmt.Errorf("duplicate invariant id %s in catalog", id)
			}
			invs[id] = inv
		}
		order = append(order, fileOrder...)
		zones = append(zones, z)
	}
	return invs, zones, order, nil
}

// parseZoneFile reads one zone catalog file, returning its zone descriptor
// (output basename + display title + ordered IDs) and the invariants it defines.
func parseZoneFile(path string) (zone, map[string]invariant, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return zone{}, nil, nil, err
	}
	defer func() { _ = f.Close() }()

	base := filepath.Base(path)
	z := zone{file: base, title: deriveTitle(base)}
	invs := map[string]invariant{}
	var order []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if m := zoneTitle.FindStringSubmatch(line); m != nil {
			z.title = m[1]
			continue
		}
		m := catalogRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := m[1]
		if _, dup := invs[id]; dup {
			return zone{}, nil, nil, fmt.Errorf("duplicate invariant id %s in %s", id, base)
		}
		invs[id] = invariant{id: id, statement: m[2]}
		order = append(order, id)
		z.ids = append(z.ids, id)
	}
	return z, invs, order, sc.Err()
}

// deriveTitle turns "01-tenancy.md" into "TENANCY" as a fallback when the file
// carries no `# Zone:` heading.
func deriveTitle(base string) string {
	if m := zoneFile.FindStringSubmatch(base); m != nil {
		return strings.ToUpper(m[2])
	}
	return base
}

// scanCoverage walks the repo for test files and maps each invariant ID to the
// sorted, de-duplicated set of covering locations (repo-relative file + tier).
func scanCoverage(root string) (map[string][]location, error) {
	hits := map[string]map[string]tier{} // id -> file -> tier (perPR wins on tie)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || isSelfPkg(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !isScannedTestFile(name) {
			return nil
		}
		anns, err := coversInFileTiered(path)
		if err != nil {
			return err
		}
		if len(anns) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, a := range anns {
			if hits[a.id] == nil {
				hits[a.id] = map[string]tier{}
			}
			// Same file, two tiers (a spec with both a smoke and a non-smoke
			// test covering one ID): the per-PR one wins — it runs in the gate.
			if existing, ok := hits[a.id][rel]; !ok || a.tier < existing {
				hits[a.id][rel] = a.tier
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := map[string][]location{}
	for id, files := range hits {
		list := make([]location, 0, len(files))
		for f, t := range files {
			list = append(list, location{file: f, tier: t})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].file < list[j].file })
		out[id] = list
	}
	return out, nil
}

// coversInFileTiered returns every invariant ID referenced by a `covers:`
// annotation in one file, each tagged with the tier it runs in. Go and vitest
// files are wholly per-PR. In a Playwright spec, a `covers:` comment sits
// directly above the `test()` it annotates (the catalog convention); the tier
// is per-PR iff that test carries an `@smoke` tag, else nightly.
func coversInFileTiered(path string) ([]annotation, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	isSpec := strings.HasSuffix(path, ".spec.ts")

	var out []annotation
	for i, line := range lines {
		m := coversLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		t := tierPerPR
		if isSpec {
			t = tierForNextTest(lines, i+1)
		}
		for _, id := range invID.FindAllString(m[1], -1) {
			out = append(out, annotation{id: id, tier: t})
		}
	}
	return out, nil
}

// tierForNextTest finds the first `test(` declaration at or after start and
// reports whether it carries an `@smoke` tag (per-PR) or not (nightly). The tag
// is part of the test() signature, so it's scanned from the `test(` line up to
// the body open (`=> {`) / the `async` keyword. A `covers:` with no following
// test() is treated as nightly — conservative: the gate won't credit it.
func tierForNextTest(lines []string, start int) tier {
	for i := start; i < len(lines); i++ {
		if !strings.Contains(lines[i], "test(") {
			continue
		}
		for j := i; j < len(lines); j++ {
			if strings.Contains(lines[j], "@smoke") {
				return tierPerPR
			}
			if strings.Contains(lines[j], "=> {") || strings.Contains(lines[j], "async") {
				return tierNightly
			}
		}
		return tierNightly
	}
	return tierNightly
}

// readLines reads a file into a slice of lines, tolerant of long lines.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// coversInFile returns every invariant ID referenced by a `covers:` annotation
// in one file.
func coversInFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var ids []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		m := coversLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		ids = append(ids, invID.FindAllString(m[1], -1)...)
	}
	return ids, sc.Err()
}

// writeCoverage regenerates docs/qa/coverage/: one file per zone that defines
// invariants (mirroring the catalog basename), plus a README.md index with the
// headline number, per-zone counts, and the uncovered/orphan findings.
func writeCoverage(dir string, invs map[string]invariant, zones []zone, order []string, coverage map[string][]location, uncovered, nightlyOnly, orphans []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Drop stale generated zone files so a renamed/removed zone doesn't linger.
	if err := pruneZoneFiles(dir, zones); err != nil {
		return err
	}

	for _, z := range zones {
		if len(z.ids) == 0 {
			continue // a seeded-but-empty zone has nothing to cover yet
		}
		if err := os.WriteFile(filepath.Join(dir, z.file), []byte(renderZone(z, invs, coverage)), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(renderIndex(zones, invs, order, coverage, uncovered, nightlyOnly, orphans)), 0o644)
}

// pruneZoneFiles removes generated NN-slug.md files in dir that no longer
// correspond to a catalog zone (README.md is preserved — it's always rewritten).
func pruneZoneFiles(dir string, zones []zone) error {
	keep := map[string]bool{}
	for _, z := range zones {
		if len(z.ids) > 0 {
			keep[z.file] = true
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !zoneFile.MatchString(e.Name()) || keep[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func renderZone(z zone, invs map[string]invariant, coverage map[string][]location) string {
	covered, perPR := 0, 0
	for _, id := range z.ids {
		if locs := coverage[id]; len(locs) > 0 {
			covered++
			if anyPerPR(locs) {
				perPR++
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Coverage: %s\n\n", z.title)
	b.WriteString("<!-- GENERATED by `make qa-matrix` — do not edit by hand. -->\n")
	fmt.Fprintf(&b, "<!-- Rows come from docs/qa/invariants/%s; the Covered-by column is\n", z.file)
	b.WriteString("     computed from `// covers:` annotations in the test suite. -->\n\n")
	fmt.Fprintf(&b, "**%d / %d** invariants in this zone have at least one covering test "+
		"(**%d** verified in the per-PR gate; the rest run nightly — _(nightly)_ below).\n\n", covered, len(z.ids), perPR)

	b.WriteString("| ID | Invariant | Covered by |\n")
	b.WriteString("|----|-----------|------------|\n")
	for _, id := range z.ids {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", id, invs[id].statement, renderCell(coverage[id]))
	}
	return b.String()
}

// renderCell formats the Covered-by column: each file in backticks, with a
// _(nightly)_ marker on locations that run only in the nightly suite.
func renderCell(locs []location) string {
	if len(locs) == 0 {
		return "— **none**"
	}
	parts := make([]string, 0, len(locs))
	for _, l := range locs {
		s := "`" + l.file + "`"
		if l.tier == tierNightly {
			s += " _(nightly)_"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "<br>")
}

func renderIndex(zones []zone, invs map[string]invariant, order []string, coverage map[string][]location, uncovered, nightlyOnly, orphans []string) string {
	var b strings.Builder
	b.WriteString("# QA coverage — index\n\n")
	b.WriteString("<!-- GENERATED by `make qa-matrix` — do not edit by hand. -->\n")
	b.WriteString("<!-- Rows come from docs/qa/invariants/; counts are computed from\n")
	b.WriteString("     `// covers:` annotations in the test suite. -->\n\n")
	perPR := len(order) - len(uncovered) - len(nightlyOnly)
	fmt.Fprintf(&b, "**%d / %d** invariants are verified in the per-PR gate "+
		"(%d covered only nightly, %d uncovered). The per-PR number is what `make qa-matrix -strict` enforces.\n\n",
		perPR, len(order), len(nightlyOnly), len(uncovered))

	b.WriteString("| Zone | Per-PR | Coverage |\n")
	b.WriteString("|----|----|----|\n")
	for _, z := range zones {
		if len(z.ids) == 0 {
			fmt.Fprintf(&b, "| %s | — seeded | — |\n", z.title)
			continue
		}
		perPRZone := 0
		for _, id := range z.ids {
			if anyPerPR(coverage[id]) {
				perPRZone++
			}
		}
		fmt.Fprintf(&b, "| %s | %d / %d | [%s](%s) |\n", z.title, perPRZone, len(z.ids), z.title, z.file)
	}

	if len(uncovered) > 0 {
		b.WriteString("\n## Uncovered invariants\n\n")
		b.WriteString("Catalogued but verified by no test — each is a QA gap.\n\n")
		for _, id := range uncovered {
			fmt.Fprintf(&b, "- **%s** — %s\n", id, invs[id].statement)
		}
	}

	if len(nightlyOnly) > 0 {
		b.WriteString("\n## Nightly-only invariants\n\n")
		b.WriteString("Covered only by a non-smoke Playwright spec, so they're verified in the\n")
		b.WriteString("nightly full suite — not in the per-PR gate. `-strict` treats these as gaps:\n")
		b.WriteString("`@smoke`-tag the covering spec or add a Go/vitest backstop to close them.\n\n")
		for _, id := range nightlyOnly {
			fmt.Fprintf(&b, "- **%s** — %s\n", id, invs[id].statement)
		}
	}

	if len(orphans) > 0 {
		b.WriteString("\n## Orphan annotations\n\n")
		b.WriteString("Referenced by a test but absent from the catalog — fix the ID or add the row.\n\n")
		for _, id := range orphans {
			fmt.Fprintf(&b, "- **%s**\n", id)
		}
	}

	return b.String()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "qa-matrix:", err)
	os.Exit(2)
}

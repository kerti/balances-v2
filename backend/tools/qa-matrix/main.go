// Command qa-matrix joins the hand-authored QA invariant catalog (the per-zone
// files under docs/qa/invariants/) against `// covers: INV-...` annotations
// scattered across the Go + TypeScript test suites, regenerates the per-zone
// coverage files under docs/qa/coverage/ plus their index, and reports any
// invariant that no test claims to verify.
//
// The catalog is the source of truth for *what must hold*; the coverage files
// are computed here so they can never lie. Advisory by default (exit 0); -strict
// turns an uncovered invariant into a non-zero exit (the future CI gate).
//
//	cd backend && go run ./tools/qa-matrix          # regenerate + report
//	cd backend && go run ./tools/qa-matrix -strict  # also fail on a gap
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

	var uncovered []string
	for _, id := range order {
		if len(coverage[id]) == 0 {
			uncovered = append(uncovered, id)
		}
	}

	outDir := filepath.Join(r, "docs", "qa", "coverage")
	rel, _ := filepath.Rel(r, outDir)
	rel = filepath.ToSlash(rel) + "/"
	if *reportFlag {
		rel = "(report-only, not written)"
	} else if err := writeCoverage(outDir, invs, zones, order, coverage, uncovered, orphans); err != nil {
		fail(fmt.Errorf("write %s: %w", outDir, err))
	}

	// Console summary.
	fmt.Printf("qa-matrix: %d/%d invariants covered → %s\n", len(order)-len(uncovered), len(order), rel)
	for _, id := range uncovered {
		fmt.Printf("  UNCOVERED %s — %s\n", id, invs[id].statement)
	}
	for _, id := range orphans {
		fmt.Printf("  ORPHAN    %s — annotated by a test but absent from the catalog\n", id)
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

	if *strictFlag && len(uncovered) > 0 {
		os.Exit(1)
	}
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
			if skipDirs[d.Name()] {
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
// sorted, de-duplicated set of repo-relative files that annotate it.
func scanCoverage(root string) (map[string][]string, error) {
	hits := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
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
		if len(ids) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, id := range ids {
			if hits[id] == nil {
				hits[id] = map[string]bool{}
			}
			hits[id][rel] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := map[string][]string{}
	for id, files := range hits {
		list := make([]string, 0, len(files))
		for f := range files {
			list = append(list, f)
		}
		sort.Strings(list)
		out[id] = list
	}
	return out, nil
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
func writeCoverage(dir string, invs map[string]invariant, zones []zone, order []string, coverage map[string][]string, uncovered, orphans []string) error {
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
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(renderIndex(zones, invs, order, coverage, uncovered, orphans)), 0o644)
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

func renderZone(z zone, invs map[string]invariant, coverage map[string][]string) string {
	covered := 0
	for _, id := range z.ids {
		if len(coverage[id]) > 0 {
			covered++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Coverage: %s\n\n", z.title)
	b.WriteString("<!-- GENERATED by `make qa-matrix` — do not edit by hand. -->\n")
	fmt.Fprintf(&b, "<!-- Rows come from docs/qa/invariants/%s; the Covered-by column is\n", z.file)
	b.WriteString("     computed from `// covers:` annotations in the test suite. -->\n\n")
	fmt.Fprintf(&b, "**%d / %d** invariants in this zone have at least one covering test.\n\n", covered, len(z.ids))

	b.WriteString("| ID | Invariant | Covered by |\n")
	b.WriteString("|----|-----------|------------|\n")
	for _, id := range z.ids {
		cell := "— **none**"
		if files := coverage[id]; len(files) > 0 {
			cell = "`" + strings.Join(files, "`<br>`") + "`"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", id, invs[id].statement, cell)
	}
	return b.String()
}

func renderIndex(zones []zone, invs map[string]invariant, order []string, coverage map[string][]string, uncovered, orphans []string) string {
	var b strings.Builder
	b.WriteString("# QA coverage — index\n\n")
	b.WriteString("<!-- GENERATED by `make qa-matrix` — do not edit by hand. -->\n")
	b.WriteString("<!-- Rows come from docs/qa/invariants/; counts are computed from\n")
	b.WriteString("     `// covers:` annotations in the test suite. -->\n\n")
	fmt.Fprintf(&b, "**%d / %d** invariants have at least one covering test.\n\n", len(order)-len(uncovered), len(order))

	b.WriteString("| Zone | Covered | Coverage |\n")
	b.WriteString("|----|----|----|\n")
	for _, z := range zones {
		if len(z.ids) == 0 {
			fmt.Fprintf(&b, "| %s | — seeded | — |\n", z.title)
			continue
		}
		covered := 0
		for _, id := range z.ids {
			if len(coverage[id]) > 0 {
				covered++
			}
		}
		fmt.Fprintf(&b, "| %s | %d / %d | [%s](%s) |\n", z.title, covered, len(z.ids), z.title, z.file)
	}

	if len(uncovered) > 0 {
		b.WriteString("\n## Uncovered invariants\n\n")
		b.WriteString("Catalogued but verified by no test — each is a QA gap.\n\n")
		for _, id := range uncovered {
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

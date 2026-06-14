// Command qa-matrix joins the hand-authored QA invariant catalog
// (docs/qa/invariants.md) against `// covers: INV-...` annotations scattered
// across the Go + TypeScript test suites, regenerates docs/qa/COVERAGE.md, and
// reports any invariant that no test claims to verify.
//
// The catalog is the source of truth for *what must hold*; the coverage column
// is computed here so it can never lie. Advisory by default (exit 0); -strict
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
)

// skipDirs are never walked — generated output, deps, and build artifacts.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	"vendor": true, "coverage": true, "playwright-report": true,
	"test-results": true,
}

type invariant struct {
	id, statement string
}

func main() {
	root := flag.String("root", "", "repo root (default: nearest ancestor with a .git)")
	strictFlag := flag.Bool("strict", false, "exit non-zero if any catalogued invariant is uncovered (the CI gate)")
	reportFlag := flag.Bool("report", false, "print the summary only; don't rewrite COVERAGE.md (used by `make check`)")
	gapsFlag := flag.Bool("gaps", false, "also list test files that carry no `covers:` annotation but sit in a directory where another test does (within-zone stragglers)")
	flag.Parse()

	r, err := resolveRoot(*root)
	if err != nil {
		fail(err)
	}

	catalogPath := filepath.Join(r, "docs", "qa", "invariants.md")
	invs, order, err := parseCatalog(catalogPath)
	if err != nil {
		fail(fmt.Errorf("read catalog %s: %w", catalogPath, err))
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

	outPath := filepath.Join(r, "docs", "qa", "COVERAGE.md")
	rel, _ := filepath.Rel(r, outPath)
	if *reportFlag {
		rel = "(report-only, not written)"
	} else if err := writeCoverage(outPath, invs, order, coverage, uncovered, orphans); err != nil {
		fail(fmt.Errorf("write %s: %w", outPath, err))
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
		if !strings.HasSuffix(name, "_test.go") && !strings.HasSuffix(name, ".spec.ts") {
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

// parseCatalog reads the markdown catalog, returning the invariants by ID and
// the order they appear (so the generated matrix mirrors the catalog).
func parseCatalog(path string) (map[string]invariant, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	invs := map[string]invariant{}
	var order []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := catalogRow.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		id := m[1]
		if _, dup := invs[id]; dup {
			return nil, nil, fmt.Errorf("duplicate invariant id %s in catalog", id)
		}
		invs[id] = invariant{id: id, statement: m[2]}
		order = append(order, id)
	}
	return invs, order, sc.Err()
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
		if !strings.HasSuffix(name, "_test.go") && !strings.HasSuffix(name, ".spec.ts") {
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

func writeCoverage(path string, invs map[string]invariant, order []string, coverage map[string][]string, uncovered, orphans []string) error {
	var b strings.Builder
	b.WriteString("# QA coverage matrix\n\n")
	b.WriteString("<!-- GENERATED by `make qa-matrix` — do not edit by hand. -->\n")
	b.WriteString("<!-- Rows come from docs/qa/invariants.md; the Covered-by column is\n")
	b.WriteString("     computed from `// covers:` annotations in the test suite. -->\n\n")
	fmt.Fprintf(&b, "**%d / %d** invariants have at least one covering test.\n\n",
		len(order)-len(uncovered), len(order))

	b.WriteString("| ID | Invariant | Covered by |\n")
	b.WriteString("|----|-----------|------------|\n")
	for _, id := range order {
		files := coverage[id]
		cell := "— **none**"
		if len(files) > 0 {
			cell = "`" + strings.Join(files, "`<br>`") + "`"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", id, invs[id].statement, cell)
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

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "qa-matrix:", err)
	os.Exit(2)
}

package pdf

import (
	"reflect"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestDecAmt(t *testing.T) {
	if got := decAmt("123.45"); got.String() != "123.45" {
		t.Errorf("decAmt(valid): got %s", got)
	}
	// A malformed string must degrade to zero, not panic — sums stay well-defined.
	if got := decAmt("not-a-number"); !got.IsZero() {
		t.Errorf("decAmt(invalid): got %s, want 0", got)
	}
}

func TestSum(t *testing.T) {
	ps := []Position{{Amount: "100"}, {Amount: "50.5"}, {Amount: "bad"}}
	if got := sum(ps).String(); got != "150.5" {
		t.Errorf("sum: got %s, want 150.5 (bad amount treated as 0)", got)
	}
	if got := sum(nil).String(); got != "0" {
		t.Errorf("sum(nil): got %s, want 0", got)
	}
}

func TestOwnersOfOrdersByTotalDesc(t *testing.T) {
	ps := []Position{
		{OwnerLabel: "Alice", Amount: "100"},
		{OwnerLabel: "Bob", Amount: "300"},
		{OwnerLabel: "Alice", Amount: "150"}, // Alice total 250 < Bob 300
	}
	owners := ownersOf(ps)
	if len(owners) != 2 || owners[0] != "Bob" || owners[1] != "Alice" {
		t.Errorf("ownersOf: got %v, want [Bob Alice] (by combined amount desc)", owners)
	}
}

func TestOwnerPositions(t *testing.T) {
	ps := []Position{
		{OwnerLabel: "Alice", Name: "A1"},
		{OwnerLabel: "Bob", Name: "B1"},
		{OwnerLabel: "Alice", Name: "A2"},
	}
	got := ownerPositions(ps, "Alice")
	if len(got) != 2 || got[0].Name != "A1" || got[1].Name != "A2" {
		t.Errorf("ownerPositions(Alice): got %v", got)
	}
	if len(ownerPositions(ps, "Nobody")) != 0 {
		t.Error("ownerPositions(Nobody): want empty")
	}
}

func TestPositionsFiltersAndSortsDesc(t *testing.T) {
	d := &doc{in: Input{Positions: []Position{
		{Group: "asset", Subtype: "bank_account", Name: "small", Amount: "100"},
		{Group: "asset", Subtype: "bank_account", Name: "big", Amount: "900"},
		{Group: "asset", Subtype: "property", Name: "house", Amount: "500"},
		{Group: "liability", Subtype: "personal", Name: "loan", Amount: "700"},
	}}}
	banks := d.positions("asset", "bank_account")
	if len(banks) != 2 || banks[0].Name != "big" || banks[1].Name != "small" {
		t.Errorf("positions(asset,bank_account): got %v, want [big small]", banks)
	}
	// Empty subtype matches the whole group.
	if got := d.positions("asset", ""); len(got) != 3 {
		t.Errorf("positions(asset,\"\"): got %d, want 3", len(got))
	}
}

func TestComposition(t *testing.T) {
	d := &doc{in: Input{Locale: "en-GB", Positions: []Position{
		{Group: "asset", Subtype: "bank_account", Amount: "600"},
		{Group: "asset", Subtype: "property", Amount: "400"},
		{Group: "asset", Subtype: "vehicle", Amount: "0"}, // zero total → dropped
	}}}
	slices := d.composition("asset", []string{"bank_account", "property", "vehicle"})
	if len(slices) != 2 {
		t.Fatalf("composition: got %d slices, want 2 (zero-value vehicle dropped)", len(slices))
	}
	if slices[0].Label != "Bank accounts" || slices[0].Value != 600 {
		t.Errorf("composition[0]: got %+v", slices[0])
	}
}

// covers: INV-PRESENTATION-22
func TestCategoryColorsAvoidDirectionHues(t *testing.T) {
	seen := map[[3]int]string{}
	for st, c := range categoryColors {
		if prev, dup := seen[c]; dup {
			t.Errorf("%s and %s share a colour", st, prev)
		}
		seen[c] = st
		r, g, b := float64(c[0]), float64(c[1]), float64(c[2])
		mx, mn := max(r, g, b), min(r, g, b)
		if mx == mn {
			continue
		}
		var h float64
		switch mx {
		case r:
			h = 60 * ((g - b) / (mx - mn))
		case g:
			h = 60 * (2 + (b-r)/(mx-mn))
		default:
			h = 60 * (4 + (r-g)/(mx-mn))
		}
		if h < 0 {
			h += 360
		}
		if h < 25 || h > 335 || (h > 90 && h < 165) {
			t.Errorf("%s colour %v has hue %.0f: red and green are reserved for direction (ADR-0054)", st, c, h)
		}
	}
	for _, st := range []string{"bank_account", "property", "vehicle", "personal", "institutional",
		"stock", "mutual_fund", "bond", "time_deposit", "gold"} {
		if _, ok := categoryColors[st]; !ok {
			t.Errorf("no category colour for %s", st)
		}
	}
}

func TestCompositionCarriesCategoryColor(t *testing.T) {
	d := &doc{in: Input{Locale: "en-GB", Positions: []Position{
		{Group: "investment", Subtype: "time_deposit", Amount: "10"},
	}}}
	got := d.composition("investment", []string{"time_deposit"})
	if len(got) != 1 || got[0].Color != categoryColors["time_deposit"] {
		t.Errorf("composition colour: got %+v", got)
	}
}

// Sentence case (#783): in headings, labels and subtype names only the first
// word is capitalised, matching the UI's locale catalogs.
func TestReportCopySentenceCase(t *testing.T) {
	check := func(loc, field, v string) {
		for i, w := range splitWords(v) {
			r, _ := utf8.DecodeRuneInString(w)
			if i > 0 && unicode.IsUpper(r) {
				t.Errorf("%s %s = %q: %q should not be capitalised", loc, field, v, w)
			}
		}
	}
	fields := []string{"title", "netWorth", "statistics", "assets", "liabilities", "investments",
		"receivables", "cashFlow", "charts", "currentAssets", "nonCurrentAssets", "cashIn", "cashOut",
		"income", "expenses", "netCashFlow", "fxTitle", "writeOffsTitle", "writeOffsTotal",
		"trackingChangesTitle", "trackingChangesTotal", "investmentPerf", "chartAssets",
		"chartInvestments", "chartLiabilities", "chartTrend"}
	for loc, c := range reportCatalog {
		v := reflect.ValueOf(c)
		for _, f := range fields {
			check(loc, f, v.FieldByName(f).String())
		}
		for i, r := range c.statRows {
			check(loc, "statRows", r)
			_ = i
		}
		for st, label := range subtypeLabels[loc] {
			check(loc, "subtype "+st, label)
		}
	}
}

func splitWords(s string) []string {
	var out []string
	start := -1
	for i, r := range s {
		if r == ' ' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func TestTotalLabelsSentenceCase(t *testing.T) {
	if got := copyFor("en-GB").total("Mutual funds"); got != "Total mutual funds" {
		t.Errorf("total: got %q", got)
	}
	if got := copyFor("en-GB").totalOf("Alice"); got != "Total Alice" {
		t.Errorf("totalOf keeps a name's capital: got %q", got)
	}
}

func TestSprintfPct(t *testing.T) {
	if got := sprintfPct("Gold", 12.34); got != "Gold  12.3%" {
		t.Errorf("sprintfPct: got %q, want %q", got, "Gold  12.3%")
	}
}

func TestSubtypeLabel(t *testing.T) {
	if got := subtypeLabel("id-ID", "bank_account"); got != "Rekening bank" {
		t.Errorf("id-ID bank_account: got %q", got)
	}
	// Unknown locale falls back to en-GB.
	if got := subtypeLabel("fr-FR", "bank_account"); got != "Bank accounts" {
		t.Errorf("fr-FR fallback: got %q", got)
	}
	// Unknown subtype passes through verbatim.
	if got := subtypeLabel("en-GB", "crypto"); got != "crypto" {
		t.Errorf("unknown subtype: got %q", got)
	}
}

func TestRiskLabel(t *testing.T) {
	if got := riskLabel("id-ID", "high"); got != "Risiko tinggi" {
		t.Errorf("id-ID high: got %q", got)
	}
	// Unknown locale falls back to en-GB.
	if got := riskLabel("fr-FR", "low"); got != "Low risk" {
		t.Errorf("fr-FR fallback: got %q", got)
	}
	// Unknown risk passes through verbatim.
	if got := riskLabel("en-GB", "extreme"); got != "extreme" {
		t.Errorf("unknown risk: got %q", got)
	}
}

func TestJointLabel(t *testing.T) {
	if got := JointLabel("id-ID"); got != "Bersama" {
		t.Errorf("id-ID joint: got %q", got)
	}
	if got := JointLabel("de-DE"); got != "Joint" {
		t.Errorf("fallback joint: got %q", got)
	}
}

func TestReportCopyTotal(t *testing.T) {
	if got := copyFor("en-GB").total("Assets"); got != "Total assets" {
		t.Errorf("en-GB total: got %q", got)
	}
	if got := copyFor("id-ID").total("Aset"); got != "Jumlah aset" {
		t.Errorf("id-ID total: got %q", got)
	}
}

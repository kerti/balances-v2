package pdf

import "testing"

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
	if slices[0].Label != "Bank Accounts" || slices[0].Value != 600 {
		t.Errorf("composition[0]: got %+v", slices[0])
	}
}

func TestPaletteAtWraps(t *testing.T) {
	if paletteAt(0) != paletteAt(len(chartPalette)) {
		t.Error("paletteAt must wrap modulo palette length")
	}
	if paletteAt(1) == paletteAt(0) {
		t.Error("adjacent palette entries must differ")
	}
}

func TestSprintfPct(t *testing.T) {
	if got := sprintfPct("Gold", 12.34); got != "Gold  12.3%" {
		t.Errorf("sprintfPct: got %q, want %q", got, "Gold  12.3%")
	}
}

func TestSubtypeLabel(t *testing.T) {
	if got := subtypeLabel("id-ID", "bank_account"); got != "Rekening Bank" {
		t.Errorf("id-ID bank_account: got %q", got)
	}
	// Unknown locale falls back to en-GB.
	if got := subtypeLabel("fr-FR", "bank_account"); got != "Bank Accounts" {
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
	if got := copyFor("en-GB").total("Assets"); got != "Total Assets" {
		t.Errorf("en-GB total: got %q", got)
	}
	if got := copyFor("id-ID").total("Harta"); got != "Jumlah Harta" {
		t.Errorf("id-ID total: got %q", got)
	}
}

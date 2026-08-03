package reports

import (
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// The Write-Off section is the PDF's answer to "our net worth moved and nothing
// explains it" (ADR-0052). Two things have to hold for it to do that job: it
// collapses entirely on a month that wrote nothing off, and it itemises the
// Positions whenever it renders — the term is one signed figure, so the total
// alone can read as "nothing happened".
// covers: INV-FINANCE-33
func TestBuildWriteOffs(t *testing.T) {
	t.Run("baseline month has no line", func(t *testing.T) {
		row := &db.MonthlyReport{WriteOffPositions: []byte(`[]`)}
		if got := buildWriteOffs(row); got != nil {
			t.Errorf("got %+v, want nil (derived lines suppressed on the baseline)", got)
		}
	})

	t.Run("zero with no constituents collapses", func(t *testing.T) {
		row := &db.MonthlyReport{WriteOffs: decp("0"), WriteOffPositions: []byte(`[]`)}
		if got := buildWriteOffs(row); got != nil {
			t.Errorf("got %+v, want nil — an empty section is noise on a normal month", got)
		}
	})

	t.Run("nets to zero but still itemises, largest movement first", func(t *testing.T) {
		row := &db.MonthlyReport{
			WriteOffs: decp("0"),
			WriteOffPositions: []byte(`[
				{"position_id":"11111111-1111-1111-1111-111111111111","name":"Old bike","group":"asset","subtype":"vehicle","amount":"-50"},
				{"position_id":"22222222-2222-2222-2222-222222222222","name":"Family loan","group":"liability","subtype":"personal","amount":"200"},
				{"position_id":"33333333-3333-3333-3333-333333333333","name":"Owed by a friend","group":"receivable","subtype":"","amount":"-150"}
			]`),
		}
		got := buildWriteOffs(row)
		if got == nil {
			t.Fatal("want a write-off section when constituents exist, even at a zero total")
		}
		if got.Total != "0" {
			t.Errorf("total: got %q, want 0 (the materialized figure the identity balanced against)", got.Total)
		}
		want := []struct{ label, amount string }{
			{"Family loan", "200"},
			{"Owed by a friend", "-150"},
			{"Old bike", "-50"},
		}
		if len(got.Items) != len(want) {
			t.Fatalf("items: got %d, want %d", len(got.Items), len(want))
		}
		for i, w := range want {
			if got.Items[i].Label != w.label || got.Items[i].Amount != w.amount {
				t.Errorf("item %d: got %q/%q, want %q/%q", i, got.Items[i].Label, got.Items[i].Amount, w.label, w.amount)
			}
		}
	})
}

// covers: INV-FINANCE-35
func TestBuildUnsettled(t *testing.T) {
	if got := buildUnsettled([]byte(`[]`)); len(got) != 0 {
		t.Errorf("got %d advisories, want 0", len(got))
	}
	got := buildUnsettled([]byte(`[{"position_id":"44444444-4444-4444-4444-444444444444","name":"Closed fund","group":"investment","subtype":"mutual_fund","reason":"no_proceeds"}]`))
	if len(got) != 1 || got[0].Label != "Closed fund" {
		t.Fatalf("got %+v, want one advisory labelled the investment", got)
	}
}

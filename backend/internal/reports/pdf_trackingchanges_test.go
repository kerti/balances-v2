package reports

import (
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// The Tracking Changes section is the PDF's answer to "our net worth jumped and
// we didn't earn it" (ADR-0053). It has the same two jobs as Write-Offs: it
// collapses entirely on a month where nothing crossed the edge of the book —
// which is nearly every month — and it itemises the Positions whenever it
// renders, because one signed term covering both directions can net toward zero
// while two real things happened (INV-FINANCE-38).
// covers: INV-FINANCE-38
func TestBuildTrackingChanges(t *testing.T) {
	t.Run("baseline month has no line", func(t *testing.T) {
		row := &db.MonthlyReport{TrackingChangePositions: []byte(`[]`)}
		if got := buildTrackingChanges(row); got != nil {
			t.Errorf("got %+v, want nil (derived lines suppressed on the baseline)", got)
		}
	})

	t.Run("zero with no constituents collapses", func(t *testing.T) {
		row := &db.MonthlyReport{TrackingChanges: decp("0"), TrackingChangePositions: []byte(`[]`)}
		if got := buildTrackingChanges(row); got != nil {
			t.Errorf("got %+v, want nil — an empty section is noise on a normal month", got)
		}
	})

	t.Run("an arrival and a departure net to zero but still itemise, largest first", func(t *testing.T) {
		row := &db.MonthlyReport{
			TrackingChanges: decp("0"),
			TrackingChangePositions: []byte(`[
				{"position_id":"11111111-1111-1111-1111-111111111111","name":"Old savings account","group":"asset","subtype":"bank","amount":"300"},
				{"position_id":"22222222-2222-2222-2222-222222222222","name":"Card moved out","group":"liability","subtype":"credit_card","amount":"100"},
				{"position_id":"33333333-3333-3333-3333-333333333333","name":"Brokerage left the household","group":"investment","subtype":"stock","amount":"-400"}
			]`),
		}
		got := buildTrackingChanges(row)
		if got == nil {
			t.Fatal("want a tracking-changes section when constituents exist, even at a zero total")
		}
		if got.Total != "0" {
			t.Errorf("total: got %q, want 0 (the materialized figure the identity balanced against)", got.Total)
		}
		want := []struct{ label, amount string }{
			{"Brokerage left the household", "-400"},
			{"Old savings account", "300"},
			{"Card moved out", "100"},
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

	t.Run("a one-sided month keeps the total's sign", func(t *testing.T) {
		row := &db.MonthlyReport{
			TrackingChanges: decp("-400"),
			TrackingChangePositions: []byte(`[
				{"position_id":"33333333-3333-3333-3333-333333333333","name":"Brokerage left the household","group":"investment","subtype":"stock","amount":"-400"}
			]`),
		}
		got := buildTrackingChanges(row)
		if got == nil || got.Total != "-400" {
			t.Fatalf("got %+v, want a section totalling -400 (value left the books)", got)
		}
	})
}

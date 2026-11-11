package investments_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// createTDWith creates a time deposit with an explicit principal, currency and
// rollover link, so the derivation can be driven against the figures that matter:
// a rolled-over principal, a cents currency.
func (h *handlerHarness) createTDWith(t *testing.T, name, currency, principal string, rolledFrom *uuid.UUID) *repo.TimeDeposit {
	t.Helper()
	body := map[string]any{
		"display_name":    name,
		"ownership_type":  "joint",
		"native_currency": currency,
		"bank_name":       "BCA",
		"principal":       principal,
		"interest_rate":   "4.5",
		"term_months":     12,
		"placement_date":  "2026-01-01",
		"maturity_date":   "2027-01-01",
		"rollover_policy": "auto_renew_with_interest",
		"risk_profile":    "medium",
	}
	if rolledFrom != nil {
		body["rolled_from_investment_id"] = rolledFrom.String()
	}
	rec := h.do(t, "POST", "/investments/time-deposits", body)
	requireStatus(t, rec, http.StatusCreated)
	return decodeBody[*repo.TimeDeposit](t, rec)
}

func mustDec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func requireAccrued(t *testing.T, h *handlerHarness, id uuid.UUID, month, wantAmount, wantAccrued string) {
	t.Helper()
	amount, qty, price, accrued := h.rawInvSnapshotShape(t, id, month)
	if !amount.Equal(mustDec(wantAmount)) {
		t.Errorf("%s amount: want %s, got %s", month, wantAmount, amount)
	}
	if accrued == nil || !accrued.Equal(mustDec(wantAccrued)) {
		t.Errorf("%s accrued_interest: want %s, got %v", month, wantAccrued, accrued)
	}
	if qty != nil || price != nil {
		t.Errorf("%s: quantity/price must stay null on the accrued branch, got %v / %v", month, qty, price)
	}
}

// A time deposit's snapshot takes one figure, the Value; the server stores
// accrued_interest = Value − principal in force and ignores whatever the client
// sent for it (#844). Covers create, edit, and the soft-deleted-then-rerecorded
// month, in IDR and in a cents currency, with exact decimals.
//
// covers: INV-SNAPSHOTS-17
func TestTimeDepositSnapshot_AccruedIsDerivedFromValue(t *testing.T) {
	t.Run("create omitting accrued_interest derives Value - principal", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Derived create") // principal 100,000,000
		rec := h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "101500000", "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
		requireAccrued(t, h, td.Investment.ID, "2026-05", "101500000", "1500000")
	})

	t.Run("a client-sent accrued_interest never wins", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Derived wins")
		rec := h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "101500000", "currency": "IDR", "accrued_interest": "7",
		})
		requireStatus(t, rec, http.StatusCreated)
		requireAccrued(t, h, td.Investment.ID, "2026-05", "101500000", "1500000")
	})

	t.Run("a rolled-over deposit derives from the rolled principal, not the original", func(t *testing.T) {
		h := newHarness(t)
		src := h.createTimeDeposit(t, "Rollover source") // principal 100,000,000
		// auto_renew_with_interest: principal + interest roll into the successor.
		succ := h.createTDWith(t, "Rollover successor", "IDR", "101125000", &src.Investment.ID)
		rec := h.do(t, "POST", "/investments/"+succ.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-06", "amount": "101500000", "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
		// 101,500,000 - 101,125,000; deriving from the source's principal would give 1,500,000.
		requireAccrued(t, h, succ.Investment.ID, "2026-06", "101500000", "375000")
	})

	t.Run("Value equal to principal derives zero", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Zero interest")
		requireStatus(t, h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-01", "amount": "100000000", "currency": "IDR",
		}), http.StatusCreated)
		requireAccrued(t, h, td.Investment.ID, "2026-01", "100000000", "0")
	})

	t.Run("Value below principal stores a negative interest and still saves", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Below principal")
		requireStatus(t, h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-02", "amount": "99000000", "currency": "IDR",
		}), http.StatusCreated)
		requireAccrued(t, h, td.Investment.ID, "2026-02", "99000000", "-1000000")
	})

	t.Run("a second currency keeps its cents exact", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTDWith(t, "USD deposit", "USD", "1000.5000", nil)
		requireStatus(t, h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "1234.5600", "currency": "USD",
		}), http.StatusCreated)
		requireAccrued(t, h, td.Investment.ID, "2026-05", "1234.56", "234.06")
	})

	t.Run("edit re-derives from the new Value and ignores a stale accrued_interest", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Derived edit")
		rec := h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "101500000", "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
		snap := decodeBody[db.InvestmentSnapshot](t, rec)

		requireStatus(t, h.do(t, "PATCH", "/investments/"+td.Investment.ID.String()+"/snapshots/"+snap.ID.String(), map[string]any{
			"amount": "102000000", "currency": "IDR", "accrued_interest": "1500000", // the old derived figure
		}), http.StatusOK)
		requireAccrued(t, h, td.Investment.ID, "2026-05", "102000000", "2000000")
		if n := h.rawInvSnapshotCount(t, td.Investment.ID); n != 1 {
			t.Errorf("edit must replace the month, not add a row: want 1 live snapshot, got %d", n)
		}
	})

	t.Run("a month deleted and re-recorded is derived afresh", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Rerecord")
		rec := h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "101500000", "currency": "IDR",
		})
		requireStatus(t, rec, http.StatusCreated)
		snap := decodeBody[db.InvestmentSnapshot](t, rec)
		requireStatus(t, h.do(t, "DELETE", "/investments/"+td.Investment.ID.String()+"/snapshots/"+snap.ID.String(), nil), http.StatusNoContent)
		if n := h.rawInvSnapshotCount(t, td.Investment.ID); n != 0 {
			t.Fatalf("setup: want 0 live snapshots after delete, got %d", n)
		}
		requireStatus(t, h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "101800000", "currency": "IDR",
		}), http.StatusCreated)
		requireAccrued(t, h, td.Investment.ID, "2026-05", "101800000", "1800000")
	})

	t.Run("quantity and price are still refused on a time deposit", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Wrong shape")
		requireStatus(t, h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "101500000", "currency": "IDR", "quantity": "1", "price_per_unit": "101500000",
		}), http.StatusBadRequest)
		if n := h.rawInvSnapshotCount(t, td.Investment.ID); n != 0 {
			t.Errorf("a rejected write must leave nothing behind, got %d rows", n)
		}
	})

	t.Run("the term-window boundary still applies before any derivation", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Term bound") // 2026-01..2027-01
		for _, ym := range []string{"2025-12", "2027-02"} {
			requireStatus(t, h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
				"year_month": ym, "amount": "101500000", "currency": "IDR",
			}), http.StatusBadRequest)
		}
		for _, ym := range []string{"2026-01", "2027-01"} {
			requireStatus(t, h.do(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
				"year_month": ym, "amount": "101500000", "currency": "IDR",
			}), http.StatusCreated)
			requireAccrued(t, h, td.Investment.ID, ym, "101500000", "1500000")
		}
	})

	t.Run("a bond still requires accrued_interest and stores it as given", func(t *testing.T) {
		h := newHarness(t)
		bond := h.createBond(t, "Bond stays direct")
		requireStatus(t, h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "10500000", "currency": "IDR",
		}), http.StatusBadRequest)
		requireStatus(t, h.do(t, "POST", "/investments/"+bond.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "10500000", "currency": "IDR", "accrued_interest": "500000",
		}), http.StatusCreated)
		requireAccrued(t, h, bond.Investment.ID, "2026-05", "10500000", "500000")
	})

	t.Run("another household cannot snapshot the deposit", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Alice's TD")
		bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
		rec := h.doRaw(t, "POST", "/investments/"+td.Investment.ID.String()+"/snapshots", map[string]any{
			"year_month": "2026-05", "amount": "101500000", "currency": "IDR",
		}, &bob)
		requireStatus(t, rec, http.StatusNotFound)
		if n := h.rawInvSnapshotCount(t, td.Investment.ID); n != 0 {
			t.Errorf("a cross-household write must leave nothing behind, got %d rows", n)
		}
	})
}

// The bulk accrued save derives a time deposit row's accrued_interest the same
// way, whether or not the client sends it, in a batch alongside a bond that still
// carries its own (#844).
//
// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-17
func TestTimeDepositSnapshot_BulkDerivesAccrued(t *testing.T) {
	t.Run("TD rows derive; a bond row in the same batch stays as given", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Bulk TD")
		src := h.createTimeDeposit(t, "Bulk rollover source")
		succ := h.createTDWith(t, "Bulk rollover successor", "IDR", "101125000", &src.Investment.ID)
		bond := h.createBond(t, "Bulk bond")

		rec := h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": td.Investment.ID.String(), "amount": "101500000", "currency": "IDR"},
			{"investment_id": succ.Investment.ID.String(), "amount": "101500000", "currency": "IDR", "accrued_interest": "999"},
			{"investment_id": bond.Investment.ID.String(), "amount": "50250000", "accrued_interest": "250000", "currency": "IDR"},
		})
		requireStatus(t, rec, http.StatusOK)

		requireAccrued(t, h, td.Investment.ID, "2026-05", "101500000", "1500000")
		requireAccrued(t, h, succ.Investment.ID, "2026-05", "101500000", "375000")
		requireAccrued(t, h, bond.Investment.ID, "2026-05", "50250000", "250000")
	})

	t.Run("below principal is stored negative; equal to principal is zero", func(t *testing.T) {
		h := newHarness(t)
		below := h.createTimeDeposit(t, "Bulk below")
		equal := h.createTimeDeposit(t, "Bulk equal")
		requireStatus(t, h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": below.Investment.ID.String(), "amount": "99500000", "currency": "IDR"},
			{"investment_id": equal.Investment.ID.String(), "amount": "100000000", "currency": "IDR"},
		}), http.StatusOK)
		requireAccrued(t, h, below.Investment.ID, "2026-05", "99500000", "-500000")
		requireAccrued(t, h, equal.Investment.ID, "2026-05", "100000000", "0")
	})

	t.Run("a cents currency stays exact", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTDWith(t, "Bulk USD", "USD", "1000.5000", nil)
		requireStatus(t, h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": td.Investment.ID.String(), "amount": "1234.56", "currency": "USD"},
		}), http.StatusOK)
		requireAccrued(t, h, td.Investment.ID, "2026-05", "1234.56", "234.06")
	})

	t.Run("re-saving the same Value is unchanged, a new Value replaces and re-derives", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Bulk resave")
		row := func(amount string) []map[string]any {
			return []map[string]any{{"investment_id": td.Investment.ID.String(), "amount": amount, "currency": "IDR"}}
		}
		requireStatus(t, h.bulkSaveAccrued(t, "2026-05", row("101500000")), http.StatusOK)

		rec := h.bulkSaveAccrued(t, "2026-05", row("101500000"))
		requireStatus(t, rec, http.StatusOK)
		if got := decodeBody[struct {
			Written int `json:"written"`
		}](t, rec); got.Written != 0 {
			t.Errorf("an identical Value must tally unchanged (written 0), got written %d", got.Written)
		}

		requireStatus(t, h.bulkSaveAccrued(t, "2026-05", row("102000000")), http.StatusOK)
		requireAccrued(t, h, td.Investment.ID, "2026-05", "102000000", "2000000")
		if n := h.rawInvSnapshotCount(t, td.Investment.ID); n != 1 {
			t.Errorf("want 1 live snapshot after a replace, got %d", n)
		}
	})

	t.Run("a bond row without accrued_interest rejects the whole batch", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Bulk batch TD")
		bond := h.createBond(t, "Bulk batch bond")
		requireStatus(t, h.bulkSaveAccrued(t, "2026-05", []map[string]any{
			{"investment_id": td.Investment.ID.String(), "amount": "101500000", "currency": "IDR"},
			{"investment_id": bond.Investment.ID.String(), "amount": "50250000", "currency": "IDR"},
		}), http.StatusBadRequest)
		if n := h.rawInvSnapshotCount(t, td.Investment.ID) + h.rawInvSnapshotCount(t, bond.Investment.ID); n != 0 {
			t.Errorf("a rejected batch must write nothing, got %d rows", n)
		}
	})

	t.Run("another household's deposit is ineligible and nothing is written", func(t *testing.T) {
		h := newHarness(t)
		td := h.createTimeDeposit(t, "Alice's bulk TD")
		bob := testutil.CreateHouseholdWithUser(t, db.New(h.pool), "Bob")
		rec := h.doRaw(t, "POST", "/investments/snapshots/accrued/bulk", map[string]any{
			"year_month": "2026-05",
			"rows":       []map[string]any{{"investment_id": td.Investment.ID.String(), "amount": "101500000", "currency": "IDR"}},
		}, &bob)
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		if n := h.rawInvSnapshotCount(t, td.Investment.ID); n != 0 {
			t.Errorf("a cross-household batch must write nothing, got %d rows", n)
		}
	})
}

// The entry list hands the client the principal in force of each time deposit so
// the row can show "of which interest" live; a bond carries none (#844).
//
// covers: INV-SNAPSHOTS-07
func TestTimeDepositSnapshot_EntryListCarriesPrincipal(t *testing.T) {
	h := newHarness(t)
	td := h.createTimeDeposit(t, "Entry TD")
	src := h.createTimeDeposit(t, "Entry source")
	succ := h.createTDWith(t, "Entry successor", "IDR", "101125000", &src.Investment.ID)
	bond := h.createBond(t, "Entry bond")

	rec := h.do(t, "GET", "/investments/snapshots/accrued/entry?year_month=2026-05", nil)
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[struct {
		Rows []struct {
			InvestmentID string  `json:"investment_id"`
			Principal    *string `json:"principal"`
		} `json:"rows"`
	}](t, rec)
	got := map[string]*string{}
	for _, r := range body.Rows {
		got[r.InvestmentID] = r.Principal
	}
	for id, want := range map[string]string{
		td.Investment.ID.String():   "100000000",
		src.Investment.ID.String():  "100000000",
		succ.Investment.ID.String(): "101125000",
	} {
		p, ok := got[id]
		if !ok || p == nil || !mustDec(*p).Equal(mustDec(want)) {
			t.Errorf("TD %s: want principal %s, got %v (listed %v)", id, want, p, ok)
		}
	}
	if p, ok := got[bond.Investment.ID.String()]; !ok || p != nil {
		t.Errorf("a bond must be listed with a null principal, got %v (listed %v)", p, ok)
	}
}

// The principal lookup behind the bulk derivation is household-scoped in SQL: a
// foreign household gets nothing for another household's deposit.
//
// covers: INV-TENANCY-01
func TestListTimeDepositPrincipals_Tenancy(t *testing.T) {
	h := newHarness(t)
	td := h.createTimeDeposit(t, "Tenancy TD")
	bond := h.createBond(t, "Tenancy bond")
	q := db.New(h.pool)
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	ids := []uuid.UUID{td.Investment.ID, bond.Investment.ID}

	rows, err := q.ListTimeDepositPrincipalsByInvestmentIDs(context.Background(), db.ListTimeDepositPrincipalsByInvestmentIDsParams{
		InvestmentIds: ids, HouseholdID: bob.HouseholdID,
	})
	if err != nil || len(rows) != 0 {
		t.Fatalf("foreign household: want no rows, got %+v (err %v)", rows, err)
	}
	rows, err = q.ListTimeDepositPrincipalsByInvestmentIDs(context.Background(), db.ListTimeDepositPrincipalsByInvestmentIDsParams{
		InvestmentIds: ids, HouseholdID: h.user.HouseholdID,
	})
	if err != nil || len(rows) != 1 || rows[0].InvestmentID != td.Investment.ID || !rows[0].Principal.Equal(mustDec("100000000")) {
		t.Fatalf("own household: want exactly the TD at 100000000 (a bond has no details row), got %+v (err %v)", rows, err)
	}
}

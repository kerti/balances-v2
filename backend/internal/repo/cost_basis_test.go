package repo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Pure unit tests for the avg-cost ledger helpers shared by the list-screen
// cost_basis aggregate (issue #18) and the monthly time-series endpoint (issue
// #22) — no DB. These mirror the frontend lib/costBasis.ts scenarios so the two
// implementations never drift: buy/sell proportional reduction, oversell clamp,
// fee capitalisation, income/maturity ignored, missing-shape skip, and the
// per-month sampling cursor.

func decp(s string) *decimal.Decimal { d := decimal.RequireFromString(s); return &d }

// txn builds an InvestmentTransaction with only the fields the ledger helpers
// read (type, date, amount, quantity); the others stay zero-valued.
func txn(typ string, date time.Time, amount, quantity *decimal.Decimal) db.InvestmentTransaction {
	return db.InvestmentTransaction{
		ID:              uuid.New(),
		TransactionType: typ,
		TransactionDate: date,
		Amount:          amount,
		Quantity:        quantity,
	}
}

func assertDec(t *testing.T, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(dec(want)) {
		t.Fatalf("got %s, want %s", got.String(), want)
	}
}

// covers: INV-COST-BASIS-01
func TestCostBasisFromLedger(t *testing.T) {
	jan := ym(2026, time.January)
	feb := ym(2026, time.February)
	mar := ym(2026, time.March)

	cases := []struct {
		name string
		txns []db.InvestmentTransaction
		want string
	}{
		{"empty ledger", nil, "0"},
		{
			"single buy adds cost",
			[]db.InvestmentTransaction{txn("buy", jan, decp("100"), decp("10"))},
			"100",
		},
		{
			"multiple buys accumulate",
			[]db.InvestmentTransaction{
				txn("buy", jan, decp("100"), decp("10")),
				txn("buy", feb, decp("200"), decp("10")),
			},
			"300",
		},
		{
			// avg cost 15/unit over 20 units; selling 5 removes 300*5/20 = 75.
			"sell reduces cost proportionally",
			[]db.InvestmentTransaction{
				txn("buy", jan, decp("100"), decp("10")),
				txn("buy", feb, decp("200"), decp("10")),
				txn("sell", mar, decp("90"), decp("5")),
			},
			"225",
		},
		{
			"sell entire holding zeroes cost",
			[]db.InvestmentTransaction{
				txn("buy", jan, decp("100"), decp("10")),
				txn("sell", feb, decp("120"), decp("10")),
			},
			"0",
		},
		{
			// sell qty (20) exceeds held (10): clamp to 10, cost drops to 0 (not negative).
			"oversell clamps to held quantity",
			[]db.InvestmentTransaction{
				txn("buy", jan, decp("100"), decp("10")),
				txn("sell", feb, decp("250"), decp("20")),
			},
			"0",
		},
		{
			"sell with nothing held is a no-op",
			[]db.InvestmentTransaction{txn("sell", jan, decp("50"), decp("5"))},
			"0",
		},
		{
			"fee capitalises into cost",
			[]db.InvestmentTransaction{
				txn("buy", jan, decp("100"), decp("10")),
				txn("fee", feb, decp("3"), nil),
			},
			"103",
		},
		{
			"coupon / dividend / distribution / maturity ignored",
			[]db.InvestmentTransaction{
				txn("buy", jan, decp("100"), decp("10")),
				txn("coupon", feb, decp("5"), nil),
				txn("dividend", feb, decp("7"), nil),
				txn("distribution", feb, decp("9"), nil),
				txn("maturity", mar, decp("100"), nil),
			},
			"100",
		},
		{
			"buy missing amount/quantity is skipped defensively",
			[]db.InvestmentTransaction{
				txn("buy", jan, nil, decp("10")),
				txn("buy", feb, decp("50"), nil),
			},
			"0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertDec(t, costBasisFromLedger(tc.txns), tc.want)
		})
	}
}

// Cost basis must be independent of the order the same transactions are summed,
// as long as buys precede their proportional sells (the query guarantees date
// ascending). Two buys then a partial sell must equal the same set regardless of
// the inter-buy order.
//
// covers: INV-COST-BASIS-02
func TestCostBasisFromLedger_BuyOrderIndependent(t *testing.T) {
	jan := ym(2026, time.January)
	feb := ym(2026, time.February)
	mar := ym(2026, time.March)

	a := []db.InvestmentTransaction{
		txn("buy", jan, decp("100"), decp("10")),
		txn("buy", feb, decp("200"), decp("10")),
		txn("sell", mar, decp("90"), decp("5")),
	}
	b := []db.InvestmentTransaction{
		txn("buy", jan, decp("200"), decp("10")),
		txn("buy", feb, decp("100"), decp("10")),
		txn("sell", mar, decp("90"), decp("5")),
	}
	if got := costBasisFromLedger(a); !got.Equal(costBasisFromLedger(b)) {
		t.Fatalf("order changed result: %s vs %s", got, costBasisFromLedger(b))
	}
}

// covers: INV-COST-BASIS-03
func TestCostSeriesAtMonths(t *testing.T) {
	jan := ym(2026, time.January)
	feb := ym(2026, time.February)
	mar := ym(2026, time.March)
	apr := ym(2026, time.April)

	t.Run("samples cumulative cost at each snapshot month", func(t *testing.T) {
		// buys in Jan (100) and Mar (200); sell in Apr is past the last month.
		txns := []db.InvestmentTransaction{
			txn("buy", jan, decp("100"), decp("10")),
			txn("buy", mar, decp("200"), decp("10")),
		}
		months := []time.Time{jan, feb, mar}
		got := costSeriesAtMonths(months, txns)
		if len(got) != 3 {
			t.Fatalf("got %d points, want 3", len(got))
		}
		// Jan: only the Jan buy. Feb: carry-forward (no txn). Mar: both buys.
		assertDec(t, got[0].Cost, "100")
		assertDec(t, got[1].Cost, "100")
		assertDec(t, got[2].Cost, "300")
		for i, m := range months {
			if !got[i].YearMonth.Equal(m) {
				t.Fatalf("point %d month = %s, want %s", i, got[i].YearMonth, m)
			}
		}
	})

	t.Run("transaction after the last month is excluded", func(t *testing.T) {
		txns := []db.InvestmentTransaction{
			txn("buy", jan, decp("100"), decp("10")),
			txn("buy", apr, decp("999"), decp("10")), // beyond last month (Feb)
		}
		got := costSeriesAtMonths([]time.Time{jan, feb}, txns)
		assertDec(t, got[0].Cost, "100")
		assertDec(t, got[1].Cost, "100")
	})

	t.Run("multiple transactions in one month all apply", func(t *testing.T) {
		txns := []db.InvestmentTransaction{
			txn("buy", jan, decp("100"), decp("10")),
			txn("buy", jan, decp("50"), decp("5")),
		}
		got := costSeriesAtMonths([]time.Time{jan}, txns)
		assertDec(t, got[0].Cost, "150")
	})

	t.Run("no months yields an empty, non-nil series", func(t *testing.T) {
		got := costSeriesAtMonths(nil, []db.InvestmentTransaction{txn("buy", jan, decp("100"), decp("10"))})
		if got == nil || len(got) != 0 {
			t.Fatalf("want empty non-nil series, got %#v", got)
		}
	})
}

// covers: INV-COST-BASIS-03
func TestFlatCostSeriesAtMonths(t *testing.T) {
	jan := ym(2026, time.January)
	feb := ym(2026, time.February)

	got := flatCostSeriesAtMonths([]time.Time{jan, feb}, dec("5000"))
	if len(got) != 2 {
		t.Fatalf("got %d points, want 2", len(got))
	}
	assertDec(t, got[0].Cost, "5000")
	assertDec(t, got[1].Cost, "5000")

	if empty := flatCostSeriesAtMonths(nil, dec("5000")); empty == nil || len(empty) != 0 {
		t.Fatalf("want empty non-nil series, got %#v", empty)
	}
}

func TestMonthKey(t *testing.T) {
	if got := monthKey(time.Date(2026, time.January, 31, 23, 59, 0, 0, time.UTC)); got != "2026-01" {
		t.Fatalf("got %q, want 2026-01", got)
	}
	// Lexical comparison of keys must equal calendar comparison across a year
	// boundary — the cursor in costSeriesAtMonths relies on this.
	if monthKey(ym(2025, time.December)) >= monthKey(ym(2026, time.January)) {
		t.Fatalf("2025-12 should sort before 2026-01")
	}
}

func TestGroupTransactionsByInvestment(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	jan := ym(2026, time.January)
	feb := ym(2026, time.February)

	mk := func(id uuid.UUID, date time.Time) db.InvestmentTransaction {
		tx := txn("buy", date, decp("1"), decp("1"))
		tx.InvestmentID = id
		return tx
	}
	flat := []db.InvestmentTransaction{
		mk(a, jan),
		mk(b, jan),
		mk(a, feb),
	}
	got := groupTransactionsByInvestment(flat)
	if len(got[a]) != 2 || len(got[b]) != 1 {
		t.Fatalf("bucketing wrong: a=%d b=%d", len(got[a]), len(got[b]))
	}
	// Order within a bucket is preserved (the batch query orders ascending).
	if !got[a][0].TransactionDate.Equal(jan) || !got[a][1].TransactionDate.Equal(feb) {
		t.Fatalf("bucket a lost ascending order")
	}
}

func TestTransactionAggregates(t *testing.T) {
	t.Run("empty ledger is 0 and nil", func(t *testing.T) {
		n, last := transactionAggregates(nil)
		if n != 0 || last != nil {
			t.Fatalf("got (%d, %v), want (0, nil)", n, last)
		}
	})

	t.Run("count + latest date from the last (ascending) element", func(t *testing.T) {
		jan := time.Date(2026, 1, 5, 9, 30, 0, 0, time.UTC)
		feb := time.Date(2026, 2, 9, 14, 0, 0, 0, time.UTC)
		n, last := transactionAggregates([]db.InvestmentTransaction{
			txn("buy", jan, decp("1"), decp("1")),
			txn("buy", feb, decp("1"), decp("1")),
		})
		if n != 2 {
			t.Errorf("count = %d, want 2", n)
		}
		// Latest = last element, formatted as a plain day (no time component).
		if last == nil || *last != "2026-02-09" {
			t.Errorf("last = %v, want 2026-02-09", last)
		}
	})
}

package repo

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// The settlement matrix and shape check are pure, so they are pinned here
// exhaustively rather than one DB round-trip per combination. The end-to-end
// path is covered in investment_settlement_test.go.

// covers: INV-LIFECYCLE-08
func TestSettlementTypeFor(t *testing.T) {
	cases := []struct {
		subtype, status, want string
	}{
		{"stock", StatusSold, TxnTypeSell},
		{"mutual_fund", StatusSold, TxnTypeSell},
		{"gold", StatusSold, TxnTypeSell},
		{"bond", StatusSold, TxnTypeSell},
		{"bond", StatusMatured, TxnTypeMaturity},
		{"time_deposit", StatusMatured, TxnTypeMaturity},
	}
	for _, c := range cases {
		got, err := settlementTypeFor(c.subtype, c.status)
		if err != nil {
			t.Errorf("%s/%s: unexpected error %v", c.subtype, c.status, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s/%s: got %q, want %q", c.subtype, c.status, got, c.want)
		}
	}

	// Every pair the group-level enum offers but no Transaction can express. The
	// dialog does not offer these; a raw-API caller can still ask for them.
	unsupported := []struct{ subtype, status string }{
		{"stock", StatusMatured},
		{"mutual_fund", StatusMatured},
		{"gold", StatusMatured},
		{"time_deposit", StatusSold},
		{"stock", StatusActive},
		{"mystery", StatusSold},
	}
	for _, c := range unsupported {
		if _, err := settlementTypeFor(c.subtype, c.status); !errors.Is(err, ErrInvalidLifecycle) {
			t.Errorf("%s/%s: want ErrInvalidLifecycle, got %v", c.subtype, c.status, err)
		}
	}
}

// covers: INV-LIFECYCLE-08
func TestSettlementParamsShape(t *testing.T) {
	inv := db.Investment{Subtype: "stock", NativeCurrency: "IDR"}
	at := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	d := func(s string) *decimal.Decimal {
		v := decimal.RequireFromString(s)
		return &v
	}

	t.Run("a sell derives its amount from quantity × price", func(t *testing.T) {
		p, err := settlementParams(inv, TxnTypeSell, InvestmentSettlement{
			Quantity: d("100"), PricePerUnit: d("11000"),
		}, at)
		if err != nil {
			t.Fatalf("settlementParams: %v", err)
		}
		if want := decimal.RequireFromString("1100000"); p.Amount == nil || !p.Amount.Equal(want) {
			t.Errorf("amount: got %v, want %s", p.Amount, want)
		}
		if p.PrincipalDisposition != nil || p.InterestDisposition != nil {
			t.Error("a sell must carry no dispositions")
		}
	})

	t.Run("a zero-priced sell is valid — it is the write-off escape", func(t *testing.T) {
		p, err := settlementParams(inv, TxnTypeSell, InvestmentSettlement{
			Quantity: d("100"), PricePerUnit: d("0"),
		}, at)
		if err != nil {
			t.Fatalf("settlementParams: %v", err)
		}
		if p.Amount == nil || !p.Amount.IsZero() {
			t.Errorf("amount: got %v, want 0", p.Amount)
		}
		// The quantity still leaves the position, so the cost basis closes out.
		if p.Quantity == nil || p.Quantity.IsZero() {
			t.Error("the write-off escape must still move the held quantity")
		}
	})

	t.Run("a maturity dispositions both legs to cash_out", func(t *testing.T) {
		p, err := settlementParams(inv, TxnTypeMaturity, InvestmentSettlement{
			PrincipalAmount: d("100000000"), InterestAmount: d("4500000"),
		}, at)
		if err != nil {
			t.Fatalf("settlementParams: %v", err)
		}
		if p.PrincipalDisposition == nil || *p.PrincipalDisposition != DispositionCashOut ||
			p.InterestDisposition == nil || *p.InterestDisposition != DispositionCashOut {
			t.Errorf("dispositions: got %v/%v, want both cash_out",
				p.PrincipalDisposition, p.InterestDisposition)
		}
		if p.Amount != nil || p.Quantity != nil || p.PricePerUnit != nil {
			t.Error("a maturity must carry no amount/quantity/price columns")
		}
	})

	bad := []struct {
		name    string
		txnType string
		s       InvestmentSettlement
	}{
		{"sell without a price", TxnTypeSell, InvestmentSettlement{Quantity: d("100")}},
		{"sell without a quantity", TxnTypeSell, InvestmentSettlement{PricePerUnit: d("1")}},
		{"sell carrying maturity columns", TxnTypeSell, InvestmentSettlement{
			Quantity: d("100"), PricePerUnit: d("1"), PrincipalAmount: d("1"),
		}},
		{"maturity without interest", TxnTypeMaturity, InvestmentSettlement{PrincipalAmount: d("1")}},
		{"maturity without principal", TxnTypeMaturity, InvestmentSettlement{InterestAmount: d("1")}},
		{"maturity carrying trade columns", TxnTypeMaturity, InvestmentSettlement{
			PrincipalAmount: d("1"), InterestAmount: d("0"), Quantity: d("1"),
		}},
	}
	for _, c := range bad {
		t.Run(c.name+" is rejected", func(t *testing.T) {
			if _, err := settlementParams(inv, c.txnType, c.s, at); !errors.Is(err, ErrInvalidTransactionShape) {
				t.Errorf("want ErrInvalidTransactionShape, got %v", err)
			}
		})
	}
}

package repo

import (
	"errors"
	"testing"
)

// Pure unit tests for validateInvestmentTransactionShape — the per-type column
// contract for the investment ledger (ADR-0023): a buy/sell carries
// amount+quantity+price, an income event (coupon/dividend/distribution) carries
// amount only, a fee may pair quantity+price, and a maturity carries the
// principal/interest split with both dispositions. No DB.

func sptr(s string) *string { return &s }

func TestValidateInvestmentTransactionShape(t *testing.T) {
	valid := []struct {
		name string
		p    CreateInvestmentTransactionParams
	}{
		{"buy", CreateInvestmentTransactionParams{TransactionType: TxnTypeBuy, Amount: decp("1"), Quantity: decp("1"), PricePerUnit: decp("1")}},
		{"sell", CreateInvestmentTransactionParams{TransactionType: TxnTypeSell, Amount: decp("1"), Quantity: decp("1"), PricePerUnit: decp("1")}},
		{"coupon", CreateInvestmentTransactionParams{TransactionType: TxnTypeCoupon, Amount: decp("1")}},
		{"dividend", CreateInvestmentTransactionParams{TransactionType: TxnTypeDividend, Amount: decp("1")}},
		{"distribution", CreateInvestmentTransactionParams{TransactionType: TxnTypeDistribution, Amount: decp("1")}},
		{"fee without qty/price", CreateInvestmentTransactionParams{TransactionType: TxnTypeFee, Amount: decp("1")}},
		{"fee with paired qty+price", CreateInvestmentTransactionParams{TransactionType: TxnTypeFee, Amount: decp("1"), Quantity: decp("1"), PricePerUnit: decp("1")}},
		{"maturity rolled+cash", CreateInvestmentTransactionParams{TransactionType: TxnTypeMaturity, PrincipalAmount: decp("1"), InterestAmount: decp("1"), PrincipalDisposition: sptr(DispositionRolledToNew), InterestDisposition: sptr(DispositionCashOut)}},
	}
	for _, tc := range valid {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if err := validateInvestmentTransactionShape(tc.p); err != nil {
				t.Errorf("got %v, want nil", err)
			}
		})
	}

	// Every rejection branch: each must be refused as ErrInvalidTransactionShape.
	invalid := []struct {
		name string
		p    CreateInvestmentTransactionParams
	}{
		{"buy missing quantity", CreateInvestmentTransactionParams{TransactionType: TxnTypeBuy, Amount: decp("1"), PricePerUnit: decp("1")}},
		{"buy with maturity columns", CreateInvestmentTransactionParams{TransactionType: TxnTypeBuy, Amount: decp("1"), Quantity: decp("1"), PricePerUnit: decp("1"), PrincipalAmount: decp("1")}},
		{"coupon missing amount", CreateInvestmentTransactionParams{TransactionType: TxnTypeCoupon}},
		{"coupon with quantity", CreateInvestmentTransactionParams{TransactionType: TxnTypeCoupon, Amount: decp("1"), Quantity: decp("1")}},
		{"coupon with maturity columns", CreateInvestmentTransactionParams{TransactionType: TxnTypeDividend, Amount: decp("1"), InterestDisposition: sptr(DispositionCashOut)}},
		{"fee missing amount", CreateInvestmentTransactionParams{TransactionType: TxnTypeFee}},
		{"fee with unpaired quantity", CreateInvestmentTransactionParams{TransactionType: TxnTypeFee, Amount: decp("1"), Quantity: decp("1")}},
		{"fee with maturity columns", CreateInvestmentTransactionParams{TransactionType: TxnTypeFee, Amount: decp("1"), PrincipalAmount: decp("1")}},
		{"maturity missing dispositions", CreateInvestmentTransactionParams{TransactionType: TxnTypeMaturity, PrincipalAmount: decp("1"), InterestAmount: decp("1")}},
		{"maturity invalid disposition", CreateInvestmentTransactionParams{TransactionType: TxnTypeMaturity, PrincipalAmount: decp("1"), InterestAmount: decp("1"), PrincipalDisposition: sptr("burned"), InterestDisposition: sptr(DispositionCashOut)}},
		{"maturity with amount", CreateInvestmentTransactionParams{TransactionType: TxnTypeMaturity, PrincipalAmount: decp("1"), InterestAmount: decp("1"), PrincipalDisposition: sptr(DispositionCashOut), InterestDisposition: sptr(DispositionCashOut), Amount: decp("1")}},
		{"unknown type", CreateInvestmentTransactionParams{TransactionType: "gift"}},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			if err := validateInvestmentTransactionShape(tc.p); !errors.Is(err, ErrInvalidTransactionShape) {
				t.Errorf("got %v, want ErrInvalidTransactionShape", err)
			}
		})
	}
}

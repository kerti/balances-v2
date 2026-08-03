package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// InvestmentSettlement is the capture-at-source payload of ADR-0052 §6: the
// terminal Transaction the terminate action writes in the same database
// transaction as the lifecycle flip, so an Investment can never be left holding
// nothing with no record of where its value went (issue #587).
//
// Its shape follows the subtype, exactly as that subtype's own transaction
// dialog would fill it — quantity × price_per_unit for a Sell, principal +
// interest for a Maturity — rather than one ambiguous "proceeds" scalar. Only
// the pair the resolved type needs may be set; the other pair must be nil.
//
// All-zero is the write-off escape (ADR-0052 §5): a 0-proceeds terminal
// Transaction is the truthful record of a position that lost its value, and it
// settles the §7 advisory rather than tripping it forever.
type InvestmentSettlement struct {
	Quantity        *decimal.Decimal
	PricePerUnit    *decimal.Decimal
	PrincipalAmount *decimal.Decimal
	InterestAmount  *decimal.Decimal
}

// settlementTypeFor resolves which terminal Transaction settles a termination,
// from the subtype's own transaction matrix (validateInvestmentTransactionType):
// TimeDeposit accepts only Maturity, the equity-shaped subtypes only Sell, and a
// Bond accepts either — so its terminal status picks.
//
// This is the single source of truth for the matrix in all three places it is
// enforced: the terminate dialog narrows its status dropdown to these pairs,
// UpdateInvestmentLifecycle refuses a *transition into* any other one (so the
// combination cannot be created over the API either), and settlementParams below
// fills the shape it names.
//
// A position already sitting on an unsupported pair — from a restore, an import,
// or a raw call predating the rule — is deliberately left editable; see the
// caller.
func settlementTypeFor(subtype, status string) (string, error) {
	switch subtype {
	case "time_deposit":
		if status == StatusMatured {
			return TxnTypeMaturity, nil
		}
	case "bond":
		switch status {
		case StatusMatured:
			return TxnTypeMaturity, nil
		case StatusSold:
			return TxnTypeSell, nil
		}
	case "stock", "mutual_fund", "gold":
		if status == StatusSold {
			return TxnTypeSell, nil
		}
	}
	return "", fmt.Errorf("%w: a %s termination of a %s cannot be settled by a transaction",
		ErrInvalidLifecycle, status, subtype)
}

// settlementParams turns the caller's subtype-shaped figures into the
// CreateInvestmentTransaction shape, deriving whatever the ledger stores
// redundantly. A Sell's amount is always quantity × price_per_unit — never taken
// from the caller — because amount is the single figure the return formula reads
// as cash_in (ADR-0008), and letting it disagree with the quantity leg would put
// the cash flow and the cost basis on different numbers.
func settlementParams(inv db.Investment, txnType string, s InvestmentSettlement, terminatedAt time.Time) (db.CreateInvestmentTransactionParams, error) {
	p := db.CreateInvestmentTransactionParams{
		ID:              inv.ID,
		TransactionType: txnType,
		TransactionDate: terminatedAt,
		Currency:        inv.NativeCurrency,
	}
	switch txnType {
	case TxnTypeSell:
		if s.Quantity == nil || s.PricePerUnit == nil {
			return p, fmt.Errorf("%w: settling a sale requires quantity and price_per_unit", ErrInvalidTransactionShape)
		}
		if s.PrincipalAmount != nil || s.InterestAmount != nil {
			return p, fmt.Errorf("%w: settling a sale must not carry maturity columns", ErrInvalidTransactionShape)
		}
		amount := s.Quantity.Mul(*s.PricePerUnit)
		p.Amount = &amount
		p.Quantity = s.Quantity
		p.PricePerUnit = s.PricePerUnit
	case TxnTypeMaturity:
		if s.PrincipalAmount == nil || s.InterestAmount == nil {
			return p, fmt.Errorf("%w: settling a maturity requires principal_amount and interest_amount", ErrInvalidTransactionShape)
		}
		if s.Quantity != nil || s.PricePerUnit != nil {
			return p, fmt.Errorf("%w: settling a maturity must not carry quantity or price_per_unit", ErrInvalidTransactionShape)
		}
		// Both legs leave the position for the bank. rolled_to_new is the one
		// disposition this path cannot produce — a rollover links a successor
		// Investment, which only the Maturity dialog can create (issue #27).
		cashOut := DispositionCashOut
		p.PrincipalAmount = s.PrincipalAmount
		p.InterestAmount = s.InterestAmount
		p.PrincipalDisposition = &cashOut
		p.InterestDisposition = &cashOut
	}
	return p, nil
}

// writeSettlement records the terminal Transaction for a termination, inside the
// caller's transaction. It runs only on the active → terminal edge: re-asserting
// a terminal status (an edit to the date or the note) must not book a second
// sale, and reactivating books nothing at all.
func writeSettlement(
	ctx context.Context,
	qtx *db.Queries,
	inv db.Investment,
	p LifecycleParams,
	s InvestmentSettlement,
	user, hid uuid.UUID,
) error {
	if inv.Status != StatusActive || p.Status == StatusActive || p.TerminatedAt == nil {
		return fmt.Errorf("%w: only an active position can be settled on termination", ErrPositionNotActive)
	}
	txnType, err := settlementTypeFor(inv.Subtype, p.Status)
	if err != nil {
		return err
	}
	params, err := settlementParams(inv, txnType, s, *p.TerminatedAt)
	if err != nil {
		return err
	}
	params.CreatedBy = &user
	params.HouseholdID = hid
	if _, err := qtx.CreateInvestmentTransaction(ctx, params); err != nil {
		return fmt.Errorf("settlement transaction on termination: %w", err)
	}
	return nil
}

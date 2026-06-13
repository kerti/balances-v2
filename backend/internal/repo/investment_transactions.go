package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Transaction-type constants. Mirrors the CHECK constraint enum in
// migration 00010. Subtype→type compatibility lives in
// validateInvestmentTransactionType below.
const (
	TxnTypeBuy          = "buy"
	TxnTypeSell         = "sell"
	TxnTypeCoupon       = "coupon"
	TxnTypeDividend     = "dividend"
	TxnTypeDistribution = "distribution"
	TxnTypeFee          = "fee"
	TxnTypeMaturity     = "maturity"
)

// Disposition values for Maturity transactions (ADR-0009 §"Maturity
// transaction extension").
const (
	DispositionRolledToNew = "rolled_to_new"
	DispositionCashOut     = "cash_out"
)

type CreateInvestmentTransactionParams struct {
	InvestmentID         uuid.UUID
	TransactionType      string
	TransactionDate      time.Time
	Currency             string
	Description          *string
	Amount               *decimal.Decimal
	Quantity             *decimal.Decimal
	PricePerUnit         *decimal.Decimal
	PrincipalAmount      *decimal.Decimal
	InterestAmount       *decimal.Decimal
	PrincipalDisposition *string
	InterestDisposition  *string
}

type UpdateInvestmentTransactionParams struct {
	TransactionID        uuid.UUID
	TransactionDate      time.Time
	Currency             string
	Description          *string
	Amount               *decimal.Decimal
	Quantity             *decimal.Decimal
	PricePerUnit         *decimal.Decimal
	PrincipalAmount      *decimal.Decimal
	InterestAmount       *decimal.Decimal
	PrincipalDisposition *string
	InterestDisposition  *string
}

func (r *InvestmentRepo) CreateInvestmentTransaction(ctx context.Context, p CreateInvestmentTransactionParams) (*db.InvestmentTransaction, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: p.InvestmentID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get investment for transaction: %w", err)
	}
	if err := validateInvestmentTransactionType(inv.Subtype, p.TransactionType); err != nil {
		return nil, err
	}
	if err := validateInvestmentTransactionShape(p); err != nil {
		return nil, err
	}
	// A time deposit's transactions — only the terminal Maturity event — must
	// fall inside its term window (issue #62). No-op for unbounded subtypes.
	bounds, err := timeDepositBounds(ctx, r.q, inv)
	if err != nil {
		return nil, err
	}
	if err := bounds.checkTransactionDate(p.TransactionDate); err != nil {
		return nil, err
	}
	// ADR-0009: a terminated position is closed to new activity. Maturity is
	// the canonical case — once it flips the position to 'matured' (below), no
	// further transactions may land; a position terminated via the lifecycle
	// action (sold/matured) is frozen the same way. Checked after type/shape
	// so a structurally-invalid request still gets the more specific error.
	if inv.Status != StatusActive {
		return nil, ErrPositionNotActive
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	txn, err := qtx.CreateInvestmentTransaction(ctx, db.CreateInvestmentTransactionParams{
		ID:                   p.InvestmentID,
		TransactionType:      p.TransactionType,
		TransactionDate:      p.TransactionDate,
		Currency:             p.Currency,
		Description:          p.Description,
		Amount:               p.Amount,
		Quantity:             p.Quantity,
		PricePerUnit:         p.PricePerUnit,
		PrincipalAmount:      p.PrincipalAmount,
		InterestAmount:       p.InterestAmount,
		PrincipalDisposition: p.PrincipalDisposition,
		InterestDisposition:  p.InterestDisposition,
		CreatedBy:            &user,
		HouseholdID:          hid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create investment transaction: %w", err)
	}

	// Maturity is terminal (ADR-0009): flip the position to 'matured' and set
	// terminated_at to the maturity date, atomically with the insert. The
	// disposition of principal/interest stays recorded on the transaction; the
	// status flip is what excludes the position from future net-worth months.
	if p.TransactionType == TxnTypeMaturity {
		termDate := p.TransactionDate
		if _, err := qtx.UpdateInvestmentLifecycle(ctx, db.UpdateInvestmentLifecycleParams{
			ID:           p.InvestmentID,
			HouseholdID:  hid,
			Status:       StatusMatured,
			TerminatedAt: &termDate,
			UpdatedBy:    &user,
		}); err != nil {
			return nil, fmt.Errorf("flip investment to matured: %w", err)
		}

		// Truthful close snapshot at maturity month (issue #25, reverses the
		// #17 data approach): a matured position holds 0 — the principal and
		// interest have left it for the bank (recorded separately as the
		// Maturity transaction's cash_out, ADR-0003 decoupling). Writing 0
		// here is what makes the unchanged return formula correct: with
		// prev≈principal, value→0, and cash_out=principal+interest, the
		// engine books interest only (ADR-0008). A non-zero close (#17's
		// principal+interest) left cash_out with nothing to cancel, so the
		// payout was double-counted as investment return.
		//
		// Bond + TimeDeposit are the only subtypes that accept Maturity (per
		// validateInvestmentTransactionType) and both use the accrued shape,
		// so the 0 close is amount=0 / accrued_interest=0. Upserts to win
		// over any pre-maturity snap the user took in the same month — the
		// month-end truth is a liquidated position. (The detail screen reads
		// "Matured on {date}" from the status, not a fictional P/L — #25.)
		zero := decimal.Zero
		ym := time.Date(p.TransactionDate.Year(), p.TransactionDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		asOf := p.TransactionDate
		if _, err := qtx.UpsertInvestmentSnapshot(ctx, db.UpsertInvestmentSnapshotParams{
			ID:              p.InvestmentID,
			YearMonth:       ym,
			Amount:          zero,
			Currency:        p.Currency,
			AccruedInterest: &zero,
			AsOfDate:        &asOf,
			CreatedBy:       &user,
			HouseholdID:     hid,
		}); err != nil {
			return nil, fmt.Errorf("close snapshot on maturity: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &txn, nil
}

func (r *InvestmentRepo) ListInvestmentTransactions(ctx context.Context, investmentID uuid.UUID) ([]db.InvestmentTransaction, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	return r.q.ListInvestmentTransactionsForInvestment(ctx, db.ListInvestmentTransactionsForInvestmentParams{
		InvestmentID: investmentID,
		HouseholdID:  hid,
	})
}

func (r *InvestmentRepo) UpdateInvestmentTransaction(ctx context.Context, p UpdateInvestmentTransactionParams) (*db.InvestmentTransaction, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Look up the existing row to validate the shape against its
	// (immutable) transaction_type. Cross-tenant attempts reach the first
	// ErrNotFound here since the query is household-scoped.
	existing, err := r.q.GetInvestmentTransactionByID(ctx, db.GetInvestmentTransactionByIDParams{ID: p.TransactionID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get investment transaction: %w", err)
	}
	if err := validateInvestmentTransactionShape(CreateInvestmentTransactionParams{
		TransactionType:      existing.TransactionType,
		Amount:               p.Amount,
		Quantity:             p.Quantity,
		PricePerUnit:         p.PricePerUnit,
		PrincipalAmount:      p.PrincipalAmount,
		InterestAmount:       p.InterestAmount,
		PrincipalDisposition: p.PrincipalDisposition,
		InterestDisposition:  p.InterestDisposition,
	}); err != nil {
		return nil, err
	}

	updateParams := db.UpdateInvestmentTransactionParams{
		ID:                   p.TransactionID,
		HouseholdID:          hid,
		TransactionDate:      p.TransactionDate,
		Currency:             p.Currency,
		Description:          p.Description,
		Amount:               p.Amount,
		Quantity:             p.Quantity,
		PricePerUnit:         p.PricePerUnit,
		PrincipalAmount:      p.PrincipalAmount,
		InterestAmount:       p.InterestAmount,
		PrincipalDisposition: p.PrincipalDisposition,
		InterestDisposition:  p.InterestDisposition,
		UpdatedBy:            &user,
	}

	// Non-maturity transactions carry no position-level state, so a plain
	// single-row update is the whole story.
	if existing.TransactionType != TxnTypeMaturity {
		txn, err := r.q.UpdateInvestmentTransaction(ctx, updateParams)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("update investment transaction: %w", err)
		}
		return &txn, nil
	}

	// Maturity is terminal: at create time it flipped the position to 'matured',
	// set terminated_at to the maturity date, and wrote a 0-value close snapshot
	// at the maturity month (see CreateInvestmentTransaction). Editing the date
	// must drag all three along with it, atomically — otherwise the position's
	// terminated_at and the close snapshot drift to a stale month (issue #58).
	// inv supplies the subtype + native currency the close-snapshot shape needs.
	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: existing.InvestmentID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get investment for maturity edit: %w", err)
	}
	// The edited Maturity date must still land inside the term (issue #62). For a
	// time deposit the maturity month is also where the 0-value close snapshot is
	// re-asserted below, so keeping the date in-window keeps that snapshot in-window
	// too. No-op for a Bond (unbounded — no placement_date).
	bounds, err := timeDepositBounds(ctx, r.q, inv)
	if err != nil {
		return nil, err
	}
	if err := bounds.checkTransactionDate(p.TransactionDate); err != nil {
		return nil, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	txn, err := qtx.UpdateInvestmentTransaction(ctx, updateParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update investment transaction: %w", err)
	}

	// Re-assert the terminal state at the (possibly new) maturity date. Status is
	// already 'matured'; this is idempotent on status and only moves terminated_at.
	newDate := p.TransactionDate
	if _, err := qtx.UpdateInvestmentLifecycle(ctx, db.UpdateInvestmentLifecycleParams{
		ID:           existing.InvestmentID,
		HouseholdID:  hid,
		Status:       StatusMatured,
		TerminatedAt: &newDate,
		UpdatedBy:    &user,
	}); err != nil {
		return nil, fmt.Errorf("re-flip investment to matured: %w", err)
	}

	// Relocate the 0-value close snapshot when the maturity month moved: drop the
	// old month's close (deleteCloseSnapshot only removes the zero-amount row, so
	// a real snapshot the user kept that month survives), then re-assert it at the
	// new month. Same month: the upsert just refreshes as_of_date. These are the
	// same helpers the lifecycle terminate/un-terminate path uses.
	oldDate := existing.TransactionDate
	if oldDate.Year() != newDate.Year() || oldDate.Month() != newDate.Month() {
		if err := deleteCloseSnapshot(ctx, qtx, existing.InvestmentID, oldDate, user, hid); err != nil {
			return nil, err
		}
	}
	if err := upsertCloseSnapshot(ctx, qtx, existing.InvestmentID, inv.Subtype, inv.NativeCurrency, newDate, user, hid); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &txn, nil
}

func (r *InvestmentRepo) DeleteInvestmentTransaction(ctx context.Context, transactionID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.SoftDeleteInvestmentTransaction(ctx, db.SoftDeleteInvestmentTransactionParams{
		ID:          transactionID,
		HouseholdID: hid,
		UpdatedBy:   &user,
	})
	if err != nil {
		return fmt.Errorf("soft delete investment transaction: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ValidateSeedTransaction validates one create-from-list ledger row (issue #90)
// against the subtype→type matrix and the ADR-0023 column-combo shape, the same
// two checks CreateInvestmentTransaction applies. It is exported so the import
// flow can surface a per-row error in the dry-run preview (a malformed row never
// reaches the all-or-nothing commit). p carries no InvestmentID — only the type
// and value columns are inspected.
func ValidateSeedTransaction(subtype string, p CreateInvestmentTransactionParams) error {
	if err := validateInvestmentTransactionType(subtype, p.TransactionType); err != nil {
		return err
	}
	return validateInvestmentTransactionShape(p)
}

// validateInvestmentTransactionType enforces the subtype→type matrix.
// TimeDeposit only accepts Maturity (placement lives in the Create dialog).
// Bond accepts the full equity-style trade plus Coupon and Maturity.
// Other subtypes accept their natural cash-income type.
func validateInvestmentTransactionType(subtype, txnType string) error {
	allowed := map[string]map[string]bool{
		"stock": {
			TxnTypeBuy: true, TxnTypeSell: true,
			TxnTypeDividend: true, TxnTypeFee: true,
		},
		"mutual_fund": {
			TxnTypeBuy: true, TxnTypeSell: true,
			TxnTypeDistribution: true, TxnTypeFee: true,
		},
		"bond": {
			TxnTypeBuy: true, TxnTypeSell: true,
			TxnTypeCoupon: true, TxnTypeFee: true, TxnTypeMaturity: true,
		},
		"gold": {
			TxnTypeBuy: true, TxnTypeSell: true, TxnTypeFee: true,
		},
		"time_deposit": {
			TxnTypeMaturity: true,
		},
	}
	types, ok := allowed[subtype]
	if !ok {
		return fmt.Errorf("%w: unknown subtype %q", ErrInvalidTransactionType, subtype)
	}
	if !types[txnType] {
		return fmt.Errorf("%w: %s does not accept transaction type %q", ErrInvalidTransactionType, subtype, txnType)
	}
	return nil
}

// validateInvestmentTransactionShape enforces that the value-column combo
// matches the declared transaction_type. The DB CHECK enforces this too,
// but catching here gives a friendlier error.
func validateInvestmentTransactionShape(p CreateInvestmentTransactionParams) error {
	switch p.TransactionType {
	case TxnTypeBuy, TxnTypeSell:
		if p.Amount == nil || p.Quantity == nil || p.PricePerUnit == nil {
			return fmt.Errorf("%w: %s requires amount, quantity, and price_per_unit", ErrInvalidTransactionShape, p.TransactionType)
		}
		if p.PrincipalAmount != nil || p.InterestAmount != nil ||
			p.PrincipalDisposition != nil || p.InterestDisposition != nil {
			return fmt.Errorf("%w: %s must not have maturity columns", ErrInvalidTransactionShape, p.TransactionType)
		}
	case TxnTypeCoupon, TxnTypeDividend, TxnTypeDistribution:
		if p.Amount == nil {
			return fmt.Errorf("%w: %s requires amount", ErrInvalidTransactionShape, p.TransactionType)
		}
		if p.Quantity != nil || p.PricePerUnit != nil {
			return fmt.Errorf("%w: %s must not have quantity or price_per_unit", ErrInvalidTransactionShape, p.TransactionType)
		}
		if p.PrincipalAmount != nil || p.InterestAmount != nil ||
			p.PrincipalDisposition != nil || p.InterestDisposition != nil {
			return fmt.Errorf("%w: %s must not have maturity columns", ErrInvalidTransactionShape, p.TransactionType)
		}
	case TxnTypeFee:
		if p.Amount == nil {
			return fmt.Errorf("%w: fee requires amount", ErrInvalidTransactionShape)
		}
		// quantity and price_per_unit are optional but must be paired.
		if (p.Quantity == nil) != (p.PricePerUnit == nil) {
			return fmt.Errorf("%w: fee quantity and price_per_unit must be set together", ErrInvalidTransactionShape)
		}
		if p.PrincipalAmount != nil || p.InterestAmount != nil ||
			p.PrincipalDisposition != nil || p.InterestDisposition != nil {
			return fmt.Errorf("%w: fee must not have maturity columns", ErrInvalidTransactionShape)
		}
	case TxnTypeMaturity:
		if p.PrincipalAmount == nil || p.InterestAmount == nil ||
			p.PrincipalDisposition == nil || p.InterestDisposition == nil {
			return fmt.Errorf("%w: maturity requires principal_amount, interest_amount, and both dispositions", ErrInvalidTransactionShape)
		}
		if !isValidDisposition(*p.PrincipalDisposition) || !isValidDisposition(*p.InterestDisposition) {
			return fmt.Errorf("%w: dispositions must be %s or %s", ErrInvalidTransactionShape, DispositionRolledToNew, DispositionCashOut)
		}
		if p.Amount != nil || p.Quantity != nil || p.PricePerUnit != nil {
			return fmt.Errorf("%w: maturity must not have amount, quantity, or price_per_unit", ErrInvalidTransactionShape)
		}
	default:
		return fmt.Errorf("%w: unknown transaction type %q", ErrInvalidTransactionShape, p.TransactionType)
	}
	return nil
}

func isValidDisposition(d string) bool {
	return d == DispositionRolledToNew || d == DispositionCashOut
}

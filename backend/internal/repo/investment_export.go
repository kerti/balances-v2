package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// InvestmentExportCommon is the half of an investment export that is identical
// across every subtype: the human-facing values for the two id-typed Detail
// fields (owner email, tag name — resolved per the Detail-sheet conventions),
// the full snapshot history, and the full transaction ledger (ADR-0023).
type InvestmentExportCommon struct {
	OwnerEmail   string // sole_owner's email; "" for joint
	TagName      string // resolved tag name; "" when untagged
	Snapshots    []db.InvestmentSnapshot
	Transactions []db.InvestmentTransaction
}

// StockExport / MutualFundExport / BondExport / GoldExport / TimeDepositExport
// pair a subtype aggregate (its core row + extension details) with the shared
// export half above. One per subtype so the handler can build the
// subtype-specific Detail sheet from the typed details.
type StockExport struct {
	Stock Stock
	InvestmentExportCommon
}

type MutualFundExport struct {
	MutualFund MutualFund
	InvestmentExportCommon
}

type BondExport struct {
	Bond Bond
	InvestmentExportCommon
}

type GoldExport struct {
	Gold Gold
	InvestmentExportCommon
}

type TimeDepositExport struct {
	TimeDeposit TimeDeposit
	InvestmentExportCommon
}

// exportCommon resolves the owner email + tag name and gathers the snapshot
// history + transaction ledger for one investment, all household-scoped. The
// caller has already fetched (and ownership-checked) the investment via the
// subtype Get, so a cross-tenant id has 404'd before reaching here.
func (r *InvestmentRepo) exportCommon(ctx context.Context, inv db.Investment) (*InvestmentExportCommon, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	out := &InvestmentExportCommon{}

	if uid := inv.SoleOwnerUserID; uid != nil {
		user, err := r.q.GetUserByID(ctx, *uid)
		if err != nil {
			return nil, fmt.Errorf("export: resolve owner: %w", err)
		}
		out.OwnerEmail = user.Email
	}

	if tid := inv.TagID; tid != nil {
		tag, err := r.q.GetTagByID(ctx, db.GetTagByIDParams{ID: *tid, HouseholdID: hid})
		if err != nil {
			return nil, fmt.Errorf("export: resolve tag: %w", err)
		}
		out.TagName = tag.Name
	}

	snaps, err := r.q.ListInvestmentSnapshotsForInvestment(ctx, db.ListInvestmentSnapshotsForInvestmentParams{
		InvestmentID: inv.ID,
		HouseholdID:  hid,
	})
	if err != nil {
		return nil, fmt.Errorf("export: list snapshots: %w", err)
	}
	out.Snapshots = snaps

	txns, err := r.q.ListInvestmentTransactionsForInvestment(ctx, db.ListInvestmentTransactionsForInvestmentParams{
		InvestmentID: inv.ID,
		HouseholdID:  hid,
	})
	if err != nil {
		return nil, fmt.Errorf("export: list transactions: %w", err)
	}
	out.Transactions = txns

	return out, nil
}

func (r *InvestmentRepo) ExportStock(ctx context.Context, id uuid.UUID) (*StockExport, error) {
	stock, err := r.GetStock(ctx, id)
	if err != nil {
		return nil, err
	}
	common, err := r.exportCommon(ctx, stock.Investment)
	if err != nil {
		return nil, err
	}
	return &StockExport{Stock: *stock, InvestmentExportCommon: *common}, nil
}

func (r *InvestmentRepo) ExportMutualFund(ctx context.Context, id uuid.UUID) (*MutualFundExport, error) {
	mf, err := r.GetMutualFund(ctx, id)
	if err != nil {
		return nil, err
	}
	common, err := r.exportCommon(ctx, mf.Investment)
	if err != nil {
		return nil, err
	}
	return &MutualFundExport{MutualFund: *mf, InvestmentExportCommon: *common}, nil
}

func (r *InvestmentRepo) ExportBond(ctx context.Context, id uuid.UUID) (*BondExport, error) {
	bond, err := r.GetBond(ctx, id)
	if err != nil {
		return nil, err
	}
	common, err := r.exportCommon(ctx, bond.Investment)
	if err != nil {
		return nil, err
	}
	return &BondExport{Bond: *bond, InvestmentExportCommon: *common}, nil
}

func (r *InvestmentRepo) ExportGold(ctx context.Context, id uuid.UUID) (*GoldExport, error) {
	gold, err := r.GetGold(ctx, id)
	if err != nil {
		return nil, err
	}
	common, err := r.exportCommon(ctx, gold.Investment)
	if err != nil {
		return nil, err
	}
	return &GoldExport{Gold: *gold, InvestmentExportCommon: *common}, nil
}

func (r *InvestmentRepo) ExportTimeDeposit(ctx context.Context, id uuid.UUID) (*TimeDepositExport, error) {
	td, err := r.GetTimeDeposit(ctx, id)
	if err != nil {
		return nil, err
	}
	common, err := r.exportCommon(ctx, td.Investment)
	if err != nil {
		return nil, err
	}
	return &TimeDepositExport{TimeDeposit: *td, InvestmentExportCommon: *common}, nil
}

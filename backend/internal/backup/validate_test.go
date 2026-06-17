package backup

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// ptrID returns a pointer to a fresh random UUID — used to forge an optional FK
// that resolves to nothing in the payload (a dangling reference).
func ptrID() *uuid.UUID {
	x := uuid.New()
	return &x
}

// fullValidEnv builds an in-memory, fully-wired Envelope with exactly one valid
// row of every section, all foreign keys resolving within the payload. It is the
// clean baseline the graph cases below poison one edge at a time; on its own it
// must pass validateGraph. No database — validateGraph is pure over the payload.
func fullValidEnv() *Envelope {
	hid := uuid.New()
	uid := uuid.New()
	tid := uuid.New()
	aid := uuid.New() // asset
	iid := uuid.New() // investment
	lid := uuid.New() // liability
	rid := uuid.New() // receivable

	return &Envelope{
		FormatVersion: FormatVersion,
		Household: HouseholdData{
			Household: db.Household{ID: hid},
			Users:     []db.User{{ID: uid, HouseholdID: hid}},
			Tags:      []db.Tag{{ID: tid, HouseholdID: hid}},

			Assets:      []db.Asset{{ID: aid, HouseholdID: hid, SoleOwnerUserID: &uid, TagID: &tid}},
			Investments: []db.Investment{{ID: iid, HouseholdID: hid, SoleOwnerUserID: &uid, TagID: &tid}},
			Liabilities: []db.Liability{{ID: lid, HouseholdID: hid, SoleOwnerUserID: &uid, TagID: &tid}},
			Receivables: []db.Receivable{{ID: rid, HouseholdID: hid, SoleOwnerUserID: &uid, TagID: &tid}},

			BankAccounts: []db.BankAccountDetail{{AssetID: aid}},
			Properties:   []db.PropertyDetail{{AssetID: aid}},
			Vehicles:     []db.VehicleDetail{{AssetID: aid}},
			Stocks:       []db.StockDetail{{InvestmentID: iid}},
			MutualFunds:  []db.MutualFundDetail{{InvestmentID: iid}},
			Bonds:        []db.BondDetail{{InvestmentID: iid}},
			Golds:        []db.GoldDetail{{InvestmentID: iid}},
			TimeDeposits: []db.TimeDepositDetail{{InvestmentID: iid}},

			AssetSnapshots:         []db.AssetSnapshot{{ID: uuid.New(), AssetID: aid}},
			LiabilitySnapshots:     []db.LiabilitySnapshot{{ID: uuid.New(), LiabilityID: lid}},
			ReceivableSnapshots:    []db.ReceivableSnapshot{{ID: uuid.New(), ReceivableID: rid}},
			InvestmentSnapshots:    []db.InvestmentSnapshot{{ID: uuid.New(), InvestmentID: iid}},
			InvestmentTransactions: []db.InvestmentTransaction{{ID: uuid.New(), InvestmentID: iid}},

			Income:  []db.Income{{ID: uuid.New(), HouseholdID: hid, SoleOwnerUserID: &uid}},
			FxRates: []db.FxRate{{ID: uuid.New(), HouseholdID: hid}},
		},
	}
}

// covers: INV-BACKUP-08, INV-BACKUP-10
func TestValidateGraph(t *testing.T) {
	t.Run("a fully-wired payload passes", func(t *testing.T) {
		if err := validateGraph(fullValidEnv()); err != nil {
			t.Fatalf("validateGraph(fullValidEnv) = %v, want nil", err)
		}
	})

	// Each case corrupts exactly one edge of an otherwise-clean payload — a
	// cross-household row (cross-tenant bleed, INV-BACKUP-10) or a foreign key
	// that resolves to nothing (dangling, INV-BACKUP-08). Every one must be
	// caught before any load touches the database.
	other := uuid.New() // a household id that is not the backup's
	cases := []struct {
		name    string
		corrupt func(d *HouseholdData)
	}{
		// Wrong-household (the wrongHousehold branch) across every top-level table.
		{"user in another household", func(d *HouseholdData) { d.Users[0].HouseholdID = other }},
		{"tag in another household", func(d *HouseholdData) { d.Tags[0].HouseholdID = other }},
		{"asset in another household", func(d *HouseholdData) { d.Assets[0].HouseholdID = other }},
		{"investment in another household", func(d *HouseholdData) { d.Investments[0].HouseholdID = other }},
		{"liability in another household", func(d *HouseholdData) { d.Liabilities[0].HouseholdID = other }},
		{"receivable in another household", func(d *HouseholdData) { d.Receivables[0].HouseholdID = other }},
		{"income in another household", func(d *HouseholdData) { d.Income[0].HouseholdID = other }},
		{"fx_rate in another household", func(d *HouseholdData) { d.FxRates[0].HouseholdID = other }},

		// Dangling owner/tag on each position type.
		{"asset dangling owner", func(d *HouseholdData) { d.Assets[0].SoleOwnerUserID = ptrID() }},
		{"asset dangling tag", func(d *HouseholdData) { d.Assets[0].TagID = ptrID() }},
		{"investment dangling owner", func(d *HouseholdData) { d.Investments[0].SoleOwnerUserID = ptrID() }},
		{"investment dangling tag", func(d *HouseholdData) { d.Investments[0].TagID = ptrID() }},
		{"investment dangling rolled-from", func(d *HouseholdData) { d.Investments[0].RolledFromInvestmentID = ptrID() }},
		{"liability dangling owner", func(d *HouseholdData) { d.Liabilities[0].SoleOwnerUserID = ptrID() }},
		{"liability dangling tag", func(d *HouseholdData) { d.Liabilities[0].TagID = ptrID() }},
		{"receivable dangling owner", func(d *HouseholdData) { d.Receivables[0].SoleOwnerUserID = ptrID() }},
		{"receivable dangling tag", func(d *HouseholdData) { d.Receivables[0].TagID = ptrID() }},
		{"income dangling owner", func(d *HouseholdData) { d.Income[0].SoleOwnerUserID = ptrID() }},

		// Detail rows pointing at no position.
		{"bank_account dangling asset", func(d *HouseholdData) { d.BankAccounts[0].AssetID = uuid.New() }},
		{"property dangling asset", func(d *HouseholdData) { d.Properties[0].AssetID = uuid.New() }},
		{"vehicle dangling asset", func(d *HouseholdData) { d.Vehicles[0].AssetID = uuid.New() }},
		{"stock dangling investment", func(d *HouseholdData) { d.Stocks[0].InvestmentID = uuid.New() }},
		{"mutual_fund dangling investment", func(d *HouseholdData) { d.MutualFunds[0].InvestmentID = uuid.New() }},
		{"bond dangling investment", func(d *HouseholdData) { d.Bonds[0].InvestmentID = uuid.New() }},
		{"gold dangling investment", func(d *HouseholdData) { d.Golds[0].InvestmentID = uuid.New() }},
		{"time_deposit dangling investment", func(d *HouseholdData) { d.TimeDeposits[0].InvestmentID = uuid.New() }},

		// Snapshots and the ledger pointing at no position.
		{"asset_snapshot dangling", func(d *HouseholdData) { d.AssetSnapshots[0].AssetID = uuid.New() }},
		{"liability_snapshot dangling", func(d *HouseholdData) { d.LiabilitySnapshots[0].LiabilityID = uuid.New() }},
		{"receivable_snapshot dangling", func(d *HouseholdData) { d.ReceivableSnapshots[0].ReceivableID = uuid.New() }},
		{"investment_snapshot dangling", func(d *HouseholdData) { d.InvestmentSnapshots[0].InvestmentID = uuid.New() }},
		{"investment_transaction dangling", func(d *HouseholdData) { d.InvestmentTransactions[0].InvestmentID = uuid.New() }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := fullValidEnv()
			tc.corrupt(&env.Household)
			if err := validateGraph(env); !errors.Is(err, ErrValidationFailed) {
				t.Errorf("validateGraph = %v, want ErrValidationFailed", err)
			}
		})
	}
}

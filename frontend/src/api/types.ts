// Base shapes (field names + null-vs-non-null) for the sqlc/repo-backed types
// below are generated from the Go source — see api/generated.types.ts and
// backend/tools/gen-ts-types (issue #365). `make backend-gen-ts-types`
// regenerates that file; CI fails if it's stale.
//
// Enum-shaped columns (subtype, status, ownership_type, ...) come out of the
// generator as plain `string`: sqlc collapses CHECK-constraint columns to a
// bare Go `string`, so there's no Go-side enum type to read literal values
// from. Every type below that has one or more such fields re-narrows them to
// a string-literal union via `Omit<Generated.X, "field"> & { field: ... }` —
// that's the one part of the mirror still hand-maintained, and the reason
// this file isn't just a re-export of generated.types.ts. Types with no enum
// fields are plain aliases.
import type * as Generated from "./generated.types";

// ----- tags (ADR-0028) --------------------------------------------------

// The position groups a Tag can attach to; the assign endpoint switches on
// this. Mirrors repo.TagGroup. Income is excluded (flow event, not a Position).
export type TagGroup = "asset" | "liability" | "receivable" | "investment";

export type Tag = Generated.Tag;

// One row of the breakdown report: the summed most-recent-snapshot value of a
// (tag, group, currency) cell. tag_id null is the Untagged bucket; `total` is
// a decimal string. Liabilities ride back under group='liability' so the
// report can render them as their own negative slice.
export type TagBreakdownRow = {
  tag_id: string | null;
  group: TagGroup;
  currency: string;
  total: string;
};

export type Asset = Omit<Generated.Asset, "subtype" | "ownership_type" | "status"> & {
  subtype: "bank_account" | "property" | "vehicle";
  ownership_type: "sole" | "joint";
  status: "active" | "closed" | "sold" | "disposed";
};

// ----- shared snapshot --------------------------------------------------

export type AssetSnapshot = Generated.AssetSnapshot;

// ----- bank account -----------------------------------------------------

export type BankAccountDetails = Omit<Generated.BankAccountDetails, "account_type"> & {
  account_type: "savings" | "current" | "other";
};

export type BankAccount = {
  asset: Asset;
  details: BankAccountDetails;
};

export type BankAccountListItem = {
  asset: Asset;
  details: BankAccountDetails;
  latest_snapshot: AssetSnapshot | null;
};

// ----- property ---------------------------------------------------------

export type PropertyDetails = Omit<Generated.PropertyDetails, "property_type"> & {
  property_type: "house" | "apartment" | "land" | "commercial";
};

export type Property = {
  asset: Asset;
  details: PropertyDetails;
};

export type PropertyListItem = {
  asset: Asset;
  details: PropertyDetails;
  latest_snapshot: AssetSnapshot | null;
};

// ----- vehicle ----------------------------------------------------------

export type VehicleDetails = Omit<Generated.VehicleDetails, "vehicle_type"> & {
  vehicle_type: "car" | "motorcycle" | "other";
};

export type Vehicle = {
  asset: Asset;
  details: VehicleDetails;
};

export type VehicleListItem = {
  asset: Asset;
  details: VehicleDetails;
  latest_snapshot: AssetSnapshot | null;
};

// ----- liability --------------------------------------------------------

export type Liability = Omit<Generated.Liability, "subtype" | "ownership_type" | "status"> & {
  subtype: "personal" | "institutional";
  ownership_type: "sole" | "joint";
  status: "active" | "paid_off" | "forgiven" | "written_off";
};

export type LiabilitySnapshot = Generated.LiabilitySnapshot;

export type LiabilityListItem = {
  liability: Liability;
  latest_snapshot: LiabilitySnapshot | null;
};

// ----- receivable -------------------------------------------------------

export type Receivable = Omit<Generated.Receivable, "ownership_type" | "status"> & {
  ownership_type: "sole" | "joint";
  status: "active" | "collected" | "written_off";
};

export type ReceivableSnapshot = Generated.ReceivableSnapshot;

export type ReceivableListItem = {
  receivable: Receivable;
  latest_snapshot: ReceivableSnapshot | null;
};

// ----- investment ------------------------------------------------------

export type InvestmentSubtype = "stock" | "mutual_fund" | "gold" | "bond" | "time_deposit";

// Risk profile (migration 00018) — user's classification of the position's
// risk. Forced manual choice on create (no default) so the user thinks; mutable
// post-create. Drives a list-row shield-icon badge and chip-bar filter.
export type RiskProfile = "low" | "medium" | "high";

// rolled_from_investment_id is set when this position was rolled over from a
// matured one (issue #29) — seeds a future rollover-chain view; null for a
// fresh position.
export type Investment = Omit<
  Generated.Investment,
  "subtype" | "ownership_type" | "risk_profile" | "status"
> & {
  subtype: InvestmentSubtype;
  ownership_type: "sole" | "joint";
  risk_profile: RiskProfile;
  status: "active" | "sold" | "matured";
};

// One snapshot table per ADR-0022. quantity + price_per_unit are populated
// for stock/mutual_fund/gold; accrued_interest is populated for
// bond/time_deposit (M4.3b). The repo validates which combo is valid based
// on the parent investment's subtype.
export type InvestmentSnapshot = Generated.InvestmentSnapshot;

export type StockDetails = Generated.StockDetails;

export type Stock = {
  investment: Investment;
  details: StockDetails;
};

export type StockListItem = {
  investment: Investment;
  details: StockDetails;
  latest_snapshot: InvestmentSnapshot | null;
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string;
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number;
  last_transaction_date: string | null;
};

// Global fund-type taxonomy (issue #20): the four universal ICI/Morningstar
// asset classes + the structural wrappers households name + an `other` tail.
// Mirrors the DB CHECK in migration 00023.
export type MutualFundType =
  | "money_market"
  | "fixed_income"
  | "equity"
  | "mixed"
  | "index"
  | "etf"
  | "target_date"
  | "commodity"
  | "other";

export type MutualFundDetails = Omit<Generated.MutualFundDetails, "fund_type"> & {
  fund_type: MutualFundType;
};

export type MutualFund = {
  investment: Investment;
  details: MutualFundDetails;
};

export type MutualFundListItem = {
  investment: Investment;
  details: MutualFundDetails;
  latest_snapshot: InvestmentSnapshot | null;
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string;
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number;
  last_transaction_date: string | null;
};

export type GoldDetails = Omit<Generated.GoldDetails, "form"> & {
  form: "bar" | "coin" | "digital" | "jewelry";
};

export type Gold = {
  investment: Investment;
  details: GoldDetails;
};

export type GoldListItem = {
  investment: Investment;
  details: GoldDetails;
  latest_snapshot: InvestmentSnapshot | null;
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string;
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number;
  last_transaction_date: string | null;
};

export type BondType = "govt_primary" | "secondary_market";
export type CouponFrequency = "monthly" | "quarterly" | "semi_annual" | "annual";
// Does the coupon pay out to the bank account or accrue inside the instrument
// (#66)? Drives the accrued-interest snapshot form's default + copy.
export type CouponDisposition = "pays_out" | "accrues";

export type BondDetails = Omit<
  Generated.BondDetails,
  "bond_type" | "coupon_frequency" | "coupon_disposition"
> & {
  bond_type: BondType;
  coupon_frequency: CouponFrequency;
  coupon_disposition: CouponDisposition;
};

export type Bond = {
  investment: Investment;
  details: BondDetails;
  // Held nominal derived from the ledger (issue #27): (Σ buy_qty − Σ sell_qty)
  // × 1,000,000. Replaces the dropped bond_details.face_value scalar.
  outstanding_face: string;
};

export type BondListItem = {
  investment: Investment;
  details: BondDetails;
  latest_snapshot: InvestmentSnapshot | null;
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string;
  // Held nominal derived from the ledger (issue #27).
  outstanding_face: string;
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number;
  last_transaction_date: string | null;
};

export type RolloverPolicy = "auto_renew_principal" | "auto_renew_with_interest" | "no_rollover";

export type TimeDepositDetails = Omit<Generated.TimeDepositDetails, "rollover_policy"> & {
  rollover_policy: RolloverPolicy;
};

// Minimal pointer to a rollover-chain neighbour — enough to render a link
// (issue #29).
export type RolloverRef = Generated.RolloverRef;

export type TimeDeposit = {
  investment: Investment;
  details: TimeDepositDetails;
  // Immediate rollover-chain neighbours, derived backend-side (issue #29).
  // rolled_from = the matured deposit this one redeployed; rolled_to = the live
  // deposit rolled over from this one (a non-null rolled_to suppresses the
  // rollover callout). Both null for a fresh deposit.
  rolled_from: RolloverRef | null;
  rolled_to: RolloverRef | null;
};

export type TimeDepositListItem = {
  investment: Investment;
  details: TimeDepositDetails;
  latest_snapshot: InvestmentSnapshot | null;
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string;
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number;
  last_transaction_date: string | null;
};

// ----- investment transaction (M4.4) ------------------------------------

export type TransactionType =
  "buy" | "sell" | "coupon" | "dividend" | "distribution" | "fee" | "maturity";

export type Disposition = "rolled_to_new" | "cash_out";

// Single polymorphic transaction row. The repo enforces subtype→type
// compatibility; the DB CHECK enforces type→shape integrity (per
// migration 00010). Frontend reads fields conditionally based on
// transaction_type — fields irrelevant to the type are null.
export type InvestmentTransaction = Omit<
  Generated.InvestmentTransaction,
  "transaction_type" | "principal_disposition" | "interest_disposition"
> & {
  transaction_type: TransactionType;
  principal_disposition: Disposition | null;
  interest_disposition: Disposition | null;
};

// ----- household members ------------------------------------------------
//
// Returned by GET /api/household/members. Public shape only — no google_sub,
// no audit columns. Used by the sole-owner picker on Income create/edit
// (M4.5); will be reused by position dialogs in a follow-up sweep.
//
// Not in generated.types.ts: assembled ad hoc in
// backend/internal/auth/handlers.go (handleListHouseholdMembers) — no
// backing Go struct exists to generate from (issue #365).

export type HouseholdMember = {
  id: string;
  display_name: string;
  nickname: string | null;
  email: string;
};

// ----- income (M4.5) ----------------------------------------------------

export type IncomeCategory =
  | "salary"
  | "business_income"
  | "rental_income"
  | "gift"
  | "tax_refund"
  | "insurance_payout"
  | "other";

// Regularity (migration 00017) — routine = expected and recurring (salary,
// business, rent); incidental = one-off / unexpected (gifts, refunds,
// payouts). Filter chip on the list, icon on the row. Required from the API.
export type Regularity = "routine" | "incidental";

export type Income = Omit<Generated.Income, "category" | "ownership_type" | "regularity"> & {
  category: IncomeCategory;
  ownership_type: "sole" | "joint";
  regularity: Regularity;
};

// ----- monthly report / dashboard (M5) ----------------------------------
// Slice-1 shape: net worth + group breakdowns + per-user/Joint breakdown +
// carried-forward (stale) positions. The income-statement fields (earned
// income, investment return, asset value change, living expenses) arrive with
// M5 slice 2. Decimals are strings to preserve precision (don't do arithmetic
// in the frontend beyond display — see lib/format.ts).

// Not in generated.types.ts: db.MonthlyReport stores UserBreakdowns/
// StalePositions/FxRatesUsed/MissingFx as raw JSONB ([]byte in Go), and the
// actual wire shape (reportResponse in backend/internal/reports/reports.go)
// re-serialises them as json.RawMessage rather than typed fields — so there's
// no backing Go struct for UserBreakdown/MonthlyReport/StalePosition/
// MissingFx to generate from (issue #365).
export type UserBreakdown = {
  nw: string;
  earned_income: string;
  investment_return: string;
};

export type MonthlyReport = {
  year_month: string; // ISO datetime, day always = 01
  generated_at: string | null;
  reporting_currency: string;
  nw_total: string;
  nw_assets: string;
  nw_liabilities: string; // positive magnitude; subtracted into nw_total
  nw_receivables: string;
  nw_investments: string;
  // Income statement (slice 2). Derived lines are null on the first-month
  // baseline (no prior month — ADR-0006). Per-category / per-subtype columns
  // also exist on the wire; typed here only as totals until a drill-down needs them.
  earned_income_total: string | null;
  investment_return_total: string | null;
  asset_value_change: string | null; // property + vehicle non-cash mark change
  derived_living_expenses: string | null; // signed cash-spending residual
  user_breakdowns: Record<string, UserBreakdown>; // keyed by user_id and "joint"
  stale_positions: StalePosition[]; // positions carried forward this month (#50)
  fx_rates_used: Record<string, string>; // currency -> rate applied this month
  missing_fx: MissingFx[]; // positions/flows excluded for want of a rate
};

// StalePosition is one position whose value this month was carried forward from
// an earlier snapshot rather than recorded fresh. group+subtype resolve the
// detail-page route; last_month is when the carried snapshot was recorded.
export type StalePosition = {
  position_id: string;
  name: string;
  group: "asset" | "liability" | "receivable" | "investment";
  subtype: string;
  last_month: string; // ISO datetime, day always = 01
};

export type MissingFx = {
  position_id: string | null;
  currency: string;
};

// ----- FX rates (M5 slice 3) --------------------------------------------

export type FxRate = Generated.FxRate;

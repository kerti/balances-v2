// Mirror of the backend JSON shapes. These will diverge if the backend
// changes them — any field tweaks here must be matched by an API change
// (we don't have shared codegen yet).

// ----- tags (ADR-0028) --------------------------------------------------

// The position groups a Tag can attach to; the assign endpoint switches on
// this. Mirrors repo.TagGroup. Income is excluded (flow event, not a Position).
export type TagGroup = 'asset' | 'liability' | 'receivable' | 'investment'

export type Tag = {
  id: string
  household_id: string
  name: string
  color: string
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
  deleted_at: string | null
}

// One row of the breakdown report: the summed most-recent-snapshot value of a
// (tag, group, currency) cell. tag_id null is the Untagged bucket; `total` is
// a decimal string. Liabilities ride back under group='liability' so the
// report can render them as their own negative slice.
export type TagBreakdownRow = {
  tag_id: string | null
  group: TagGroup
  currency: string
  total: string
}

export type Asset = {
  id: string
  household_id: string
  display_name: string
  description: string | null
  subtype: 'bank_account' | 'property' | 'vehicle'
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  native_currency: string
  tag_id: string | null
  status: 'active' | 'closed' | 'sold' | 'disposed'
  terminated_at: string | null
  termination_note: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

// ----- shared snapshot --------------------------------------------------

export type AssetSnapshot = {
  id: string
  asset_id: string
  year_month: string // ISO datetime, day always = 01
  amount: string // decimal as string to preserve precision
  currency: string
  as_of_date: string | null
  description: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

// ----- bank account -----------------------------------------------------

export type BankAccountDetails = {
  asset_id: string
  bank_name: string
  account_number: string
  account_type: 'savings' | 'current' | 'other'
}

export type BankAccount = {
  asset: Asset
  details: BankAccountDetails
}

export type BankAccountListItem = {
  asset: Asset
  details: BankAccountDetails
  latest_snapshot: AssetSnapshot | null
}

// ----- property ---------------------------------------------------------

export type PropertyDetails = {
  asset_id: string
  property_type: 'house' | 'apartment' | 'land' | 'commercial'
  address: string | null
  acquisition_date: string | null
  acquisition_cost: string | null
  annual_appreciation_rate: string | null
}

export type Property = {
  asset: Asset
  details: PropertyDetails
}

export type PropertyListItem = {
  asset: Asset
  details: PropertyDetails
  latest_snapshot: AssetSnapshot | null
}

// ----- vehicle ----------------------------------------------------------

export type VehicleDetails = {
  asset_id: string
  vehicle_type: 'car' | 'motorcycle' | 'other'
  make: string | null
  model: string | null
  year: number | null
  plate_number: string | null
  annual_depreciation_rate: string | null
}

export type Vehicle = {
  asset: Asset
  details: VehicleDetails
}

export type VehicleListItem = {
  asset: Asset
  details: VehicleDetails
  latest_snapshot: AssetSnapshot | null
}

// ----- liability --------------------------------------------------------

export type Liability = {
  id: string
  household_id: string
  display_name: string
  description: string | null
  subtype: 'personal' | 'institutional'
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  native_currency: string
  tag_id: string | null
  status: 'active' | 'paid_off' | 'forgiven' | 'written_off'
  terminated_at: string | null
  termination_note: string | null
  counterparty_name: string
  principal: string | null
  interest_rate: string | null
  term_months: number | null
  start_date: string | null
  maturity_date: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

export type LiabilitySnapshot = {
  id: string
  liability_id: string
  year_month: string
  amount: string
  currency: string
  as_of_date: string | null
  description: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

export type LiabilityListItem = {
  liability: Liability
  latest_snapshot: LiabilitySnapshot | null
}

// ----- receivable -------------------------------------------------------

export type Receivable = {
  id: string
  household_id: string
  display_name: string
  description: string | null
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  native_currency: string
  tag_id: string | null
  status: 'active' | 'collected' | 'written_off'
  terminated_at: string | null
  termination_note: string | null
  counterparty_name: string
  due_date: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

export type ReceivableSnapshot = {
  id: string
  receivable_id: string
  year_month: string
  amount: string
  currency: string
  as_of_date: string | null
  description: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

export type ReceivableListItem = {
  receivable: Receivable
  latest_snapshot: ReceivableSnapshot | null
}

// ----- investment ------------------------------------------------------

export type InvestmentSubtype =
  | 'stock'
  | 'mutual_fund'
  | 'gold'
  | 'bond'
  | 'time_deposit'

// Risk profile (migration 00018) — user's classification of the position's
// risk. Forced manual choice on create (no default) so the user thinks; mutable
// post-create. Drives a list-row shield-icon badge and chip-bar filter.
export type RiskProfile = 'low' | 'medium' | 'high'

export type Investment = {
  id: string
  household_id: string
  display_name: string
  description: string | null
  subtype: InvestmentSubtype
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  native_currency: string
  risk_profile: RiskProfile
  // Set when this position was rolled over from a matured one (issue #29) —
  // seeds a future rollover-chain view; null for a fresh position.
  rolled_from_investment_id: string | null
  tag_id: string | null
  status: 'active' | 'sold' | 'matured'
  terminated_at: string | null
  termination_note: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

// One snapshot table per ADR-0022. quantity + price_per_unit are populated
// for stock/mutual_fund/gold; accrued_interest is populated for
// bond/time_deposit (M4.3b). The repo validates which combo is valid based
// on the parent investment's subtype.
export type InvestmentSnapshot = {
  id: string
  investment_id: string
  year_month: string
  amount: string
  currency: string
  quantity: string | null
  price_per_unit: string | null
  accrued_interest: string | null
  as_of_date: string | null
  description: string | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

export type StockDetails = {
  investment_id: string
  ticker: string
  exchange: string
}

export type Stock = {
  investment: Investment
  details: StockDetails
}

export type StockListItem = {
  investment: Investment
  details: StockDetails
  latest_snapshot: InvestmentSnapshot | null
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number
  last_transaction_date: string | null
}

// Global fund-type taxonomy (issue #20): the four universal ICI/Morningstar
// asset classes + the structural wrappers households name + an `other` tail.
// Mirrors the DB CHECK in migration 00023.
export type MutualFundType =
  | 'money_market'
  | 'fixed_income'
  | 'equity'
  | 'mixed'
  | 'index'
  | 'etf'
  | 'target_date'
  | 'commodity'
  | 'other'

export type MutualFundDetails = {
  investment_id: string
  fund_code: string
  fund_manager: string | null
  fund_type: MutualFundType
}

export type MutualFund = {
  investment: Investment
  details: MutualFundDetails
}

export type MutualFundListItem = {
  investment: Investment
  details: MutualFundDetails
  latest_snapshot: InvestmentSnapshot | null
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number
  last_transaction_date: string | null
}

export type GoldDetails = {
  investment_id: string
  form: 'bar' | 'coin' | 'digital' | 'jewelry'
  purity: string
}

export type Gold = {
  investment: Investment
  details: GoldDetails
}

export type GoldListItem = {
  investment: Investment
  details: GoldDetails
  latest_snapshot: InvestmentSnapshot | null
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number
  last_transaction_date: string | null
}

export type BondType = 'govt_primary' | 'secondary_market'
export type CouponFrequency =
  | 'monthly'
  | 'quarterly'
  | 'semi_annual'
  | 'annual'
// Does the coupon pay out to the bank account or accrue inside the instrument
// (#66)? Drives the accrued-interest snapshot form's default + copy.
export type CouponDisposition = 'pays_out' | 'accrues'

export type BondDetails = {
  investment_id: string
  bond_type: BondType
  series_code: string | null
  issuer: string
  coupon_rate: string
  coupon_frequency: CouponFrequency
  coupon_disposition: CouponDisposition
  maturity_date: string
}

export type Bond = {
  investment: Investment
  details: BondDetails
  // Held nominal derived from the ledger (issue #27): (Σ buy_qty − Σ sell_qty)
  // × 1,000,000. Replaces the dropped bond_details.face_value scalar.
  outstanding_face: string
}

export type BondListItem = {
  investment: Investment
  details: BondDetails
  latest_snapshot: InvestmentSnapshot | null
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string
  // Held nominal derived from the ledger (issue #27).
  outstanding_face: string
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number
  last_transaction_date: string | null
}

export type RolloverPolicy =
  | 'auto_renew_principal'
  | 'auto_renew_with_interest'
  | 'no_rollover'

export type TimeDepositDetails = {
  investment_id: string
  bank_name: string
  principal: string
  interest_rate: string
  term_months: number
  placement_date: string
  maturity_date: string
  rollover_policy: RolloverPolicy
}

// Minimal pointer to a rollover-chain neighbour — enough to render a link
// (issue #29).
export type RolloverRef = {
  id: string
  display_name: string
}

export type TimeDeposit = {
  investment: Investment
  details: TimeDepositDetails
  // Immediate rollover-chain neighbours, derived backend-side (issue #29).
  // rolled_from = the matured deposit this one redeployed; rolled_to = the live
  // deposit rolled over from this one (a non-null rolled_to suppresses the
  // rollover callout). Both null for a fresh deposit.
  rolled_from: RolloverRef | null
  rolled_to: RolloverRef | null
}

export type TimeDepositListItem = {
  investment: Investment
  details: TimeDepositDetails
  latest_snapshot: InvestmentSnapshot | null
  // Avg-cost ledger replay folded into the list payload (issue #18) — the
  // headline P/L reads this instead of replaying transactions client-side.
  cost_basis: string
  // Ledger summary for the row (issue #67). last_transaction_date is
  // YYYY-MM-DD, null when there are none.
  transaction_count: number
  last_transaction_date: string | null
}

// ----- investment transaction (M4.4) ------------------------------------

export type TransactionType =
  | 'buy'
  | 'sell'
  | 'coupon'
  | 'dividend'
  | 'distribution'
  | 'fee'
  | 'maturity'

export type Disposition = 'rolled_to_new' | 'cash_out'

// Single polymorphic transaction row. The repo enforces subtype→type
// compatibility; the DB CHECK enforces type→shape integrity (per
// migration 00010). Frontend reads fields conditionally based on
// transaction_type — fields irrelevant to the type are null.
export type InvestmentTransaction = {
  id: string
  investment_id: string
  transaction_type: TransactionType
  transaction_date: string // YYYY-MM-DD
  currency: string
  description: string | null
  amount: string | null
  quantity: string | null
  price_per_unit: string | null
  principal_amount: string | null
  interest_amount: string | null
  principal_disposition: Disposition | null
  interest_disposition: Disposition | null
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

// ----- household members ------------------------------------------------
//
// Returned by GET /api/household/members. Public shape only — no google_sub,
// no audit columns. Used by the sole-owner picker on Income create/edit
// (M4.5); will be reused by position dialogs in a follow-up sweep.

export type HouseholdMember = {
  id: string
  display_name: string
  nickname: string | null
  email: string
}

// ----- income (M4.5) ----------------------------------------------------

export type IncomeCategory =
  | 'salary'
  | 'business_income'
  | 'rental_income'
  | 'gift'
  | 'tax_refund'
  | 'insurance_payout'
  | 'other'

// Regularity (migration 00017) — routine = expected and recurring (salary,
// business, rent); incidental = one-off / unexpected (gifts, refunds,
// payouts). Filter chip on the list, icon on the row. Required from the API.
export type Regularity = 'routine' | 'incidental'

export type Income = {
  id: string
  household_id: string
  date: string // YYYY-MM-DD
  amount: string
  currency: string
  category: IncomeCategory
  description: string | null
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  regularity: Regularity
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

// ----- monthly report / dashboard (M5) ----------------------------------
// Slice-1 shape: net worth + group breakdowns + per-user/Joint breakdown +
// carried-forward (stale) positions. The income-statement fields (earned
// income, investment return, asset value change, living expenses) arrive with
// M5 slice 2. Decimals are strings to preserve precision (don't do arithmetic
// in the frontend beyond display — see lib/format.ts).

export type UserBreakdown = {
  nw: string
  earned_income: string
  investment_return: string
}

export type MonthlyReport = {
  year_month: string // ISO datetime, day always = 01
  generated_at: string | null
  reporting_currency: string
  nw_total: string
  nw_assets: string
  nw_liabilities: string // positive magnitude; subtracted into nw_total
  nw_receivables: string
  nw_investments: string
  // Income statement (slice 2). Derived lines are null on the first-month
  // baseline (no prior month — ADR-0006). Per-category / per-subtype columns
  // also exist on the wire; typed here only as totals until a drill-down needs them.
  earned_income_total: string | null
  investment_return_total: string | null
  asset_value_change: string | null // property + vehicle non-cash mark change
  derived_living_expenses: string | null // signed cash-spending residual
  user_breakdowns: Record<string, UserBreakdown> // keyed by user_id and "joint"
  stale_positions: StalePosition[] // positions carried forward this month (#50)
  fx_rates_used: Record<string, string> // currency -> rate applied this month
  missing_fx: MissingFx[] // positions/flows excluded for want of a rate
}

// StalePosition is one position whose value this month was carried forward from
// an earlier snapshot rather than recorded fresh. group+subtype resolve the
// detail-page route; last_month is when the carried snapshot was recorded.
export type StalePosition = {
  position_id: string
  name: string
  group: 'asset' | 'liability' | 'receivable' | 'investment'
  subtype: string
  last_month: string // ISO datetime, day always = 01
}

export type MissingFx = {
  position_id: string | null
  currency: string
}

// ----- FX rates (M5 slice 3) --------------------------------------------

export type FxRate = {
  id: string
  household_id: string
  year_month: string // ISO datetime, day always = 01
  currency: string
  rate: string // reporting-currency units per 1 unit of `currency`
  created_by: string | null
  created_at: string
  updated_by: string | null
  updated_at: string
}

import type { InvestmentTransaction, RiskProfile, RolloverPolicy, TimeDeposit } from "@/api/types";

// Shape of the CreateTimeDepositDialog form. Lives here (not in the dialog) so
// the rollover helper can describe a prefill without the dialog depending on
// this module's caller — the dialog imports the type back.
export type TimeDepositForm = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile | "";
  native_currency: string;
  bank_name: string;
  principal: string;
  interest_rate: string;
  term_months: string;
  placement_date: string;
  maturity_date: string;
  rollover_policy: RolloverPolicy;
};

// placement_date + term_months → maturity_date. Shared with the dialog's
// recompute-on-edit behaviour. Bank-actual maturities sometimes nudge by a day
// or two (holidays); the user can edit the computed value afterward.
export function addMonths(date: string, months: number): string {
  if (!date || Number.isNaN(months) || months <= 0) return "";
  const d = new Date(date);
  if (Number.isNaN(d.getTime())) return "";
  d.setMonth(d.getMonth() + months);
  return d.toISOString().slice(0, 10);
}

// When a TD matured with a rolled-over disposition (Q14c-iv), the rolled funds
// belong in a fresh deposit. Returns the rolled amount plus a Create-TD prefill
// (placement_date = the maturity date, principal = rolled principal + rolled
// interest), or null when nothing rolled — only cash_out, or no maturity txn.
export function maturityRolloverPrefill(
  td: TimeDeposit,
  transactions: InvestmentTransaction[] | undefined,
): { rolledAmount: number; prefill: Partial<TimeDepositForm> } | null {
  // The rolled funds already live in a successor deposit — don't nag to create
  // one that exists (issue #29). Hand-created successors stay unlinked and still
  // prompt; only helper-created ones carry the back-reference.
  if (td.rolled_to) return null;

  const maturity = (transactions ?? []).find((tx) => tx.transaction_type === "maturity");
  if (!maturity) return null;

  const rolledPrincipal =
    maturity.principal_disposition === "rolled_to_new" ? Number(maturity.principal_amount ?? 0) : 0;
  const rolledInterest =
    maturity.interest_disposition === "rolled_to_new" ? Number(maturity.interest_amount ?? 0) : 0;
  const rolledAmount = rolledPrincipal + rolledInterest;
  if (!(rolledAmount > 0)) return null;

  // The new deposit starts where the old one matured. Use the TD's scheduled
  // maturity_date (not the maturity txn's transaction_date) — that's the date
  // the funds actually became available to redeploy. The wire value is an
  // RFC3339 timestamp (Go time.Time); a <input type="date"> needs YYYY-MM-DD.
  const placement_date = td.details.maturity_date.slice(0, 10);
  return {
    rolledAmount,
    prefill: {
      display_name: td.investment.display_name,
      description: td.investment.description ?? "",
      ownership_type: td.investment.ownership_type,
      sole_owner_user_id: td.investment.sole_owner_user_id,
      risk_profile: td.investment.risk_profile,
      native_currency: td.investment.native_currency,
      bank_name: td.details.bank_name,
      principal: String(rolledAmount),
      interest_rate: td.details.interest_rate,
      term_months: String(td.details.term_months),
      placement_date,
      maturity_date: addMonths(placement_date, td.details.term_months),
      rollover_policy: td.details.rollover_policy,
    },
  };
}

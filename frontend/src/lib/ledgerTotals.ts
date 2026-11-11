// The totals line above an investment's transaction ledger (#830, generalising
// the bond-only coupon/fee totals of #14). Each subtype totals exactly the
// transaction kinds it can record (the ADR-0023 subtype -> type matrix, which
// the backend's validateInvestmentTransactionType enforces):
//
//   stock         buy sell dividend fee             -> dividends, fees
//   mutual_fund   buy sell distribution fee         -> distributions, fees
//   bond          buy sell coupon fee maturity      -> coupons, fees
//   gold          buy sell fee                      -> fees
//   time_deposit  maturity                          -> principal, interest paid
//
// Buy/Sell are not totalled: they are trades, already summarised by the headline's
// cost basis. A bond's Maturity is terminal rather than recurring income (the #14
// "yield-to-date" glance), so it is left out; a time deposit's only kind *is* the
// Maturity, so it totals that instead.
//
// Money is summed as exact decimals (lib/decimal), never as floats. Each row
// carries its own currency (the ledger formats every row in it) and amounts in
// different currencies are never added together: a total with rows in two
// currencies reports one figure per currency.
import { sumDecimals } from "@/lib/decimal";
import type { InvestmentSubtype } from "@/lib/lifecycle";
import type { InvestmentTransaction } from "@/api/types";

export type LedgerTotalKey =
  "coupon" | "dividend" | "distribution" | "fee" | "maturityPrincipal" | "maturityInterest";

export const LEDGER_TOTAL_KEYS: Record<InvestmentSubtype, LedgerTotalKey[]> = {
  stock: ["dividend", "fee"],
  mutual_fund: ["distribution", "fee"],
  bond: ["coupon", "fee"],
  gold: ["fee"],
  time_deposit: ["maturityPrincipal", "maturityInterest"],
};

export type CurrencyAmount = { currency: string; amount: string };
export type LedgerTotal = { key: LedgerTotalKey; amounts: CurrencyAmount[] };

const SOURCE: Record<
  LedgerTotalKey,
  { type: string; field: "amount" | "principal_amount" | "interest_amount" }
> = {
  coupon: { type: "coupon", field: "amount" },
  dividend: { type: "dividend", field: "amount" },
  distribution: { type: "distribution", field: "amount" },
  fee: { type: "fee", field: "amount" },
  maturityPrincipal: { type: "maturity", field: "principal_amount" },
  maturityInterest: { type: "maturity", field: "interest_amount" },
};

// One entry per key the subtype totals, in display order. A key with no rows
// totals zero in `fallbackCurrency` (the position's), so the line never drops a
// figure just because none of that kind has been recorded yet.
export function ledgerTotals(
  subtype: InvestmentSubtype,
  transactions: InvestmentTransaction[],
  fallbackCurrency: string,
): LedgerTotal[] {
  return LEDGER_TOTAL_KEYS[subtype].map((key) => {
    const { type, field } = SOURCE[key];
    const byCurrency = new Map<string, string[]>();
    for (const tx of transactions) {
      if (tx.transaction_type !== type) continue;
      const bucket = byCurrency.get(tx.currency) ?? [];
      const value = tx[field];
      if (value !== null) bucket.push(value);
      byCurrency.set(tx.currency, bucket);
    }
    const amounts = [...byCurrency.entries()].map(([currency, values]) => ({
      currency,
      amount: sumDecimals(values),
    }));
    return {
      key,
      amounts: amounts.length > 0 ? amounts : [{ currency: fallbackCurrency, amount: "0" }],
    };
  });
}

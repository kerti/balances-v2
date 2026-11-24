// The investment detail card's money summary, as label/value fields for the
// card's one InfoGrid (#839; it used to be its own stat column, ADR-0051 Phase B).
// Top to bottom, the one order every investment card shares (#994): total value,
// total cost, gain / loss, then the income received where the type earns one
// (bond coupons, stock dividends, fund distributions, time-deposit interest once
// matured; gold has none). The core then follows with the descriptor's `infoFields` (the
// type-specific facts) and the ownership row. Risk profile is not here — it
// rides the details-card header as a compact badge (via the descriptor's
// `renderHeaderBadge` slot).
//
// Gain / loss is price-only (value minus cost): income is shown on its own row,
// from the same ledger totals as the transaction toolbar, never folded in.
//
// **Terminated-position short-circuit.** A terminated position holds a
// truthful 0-value close snapshot at its termination month (#25): the
// cash has left the position for the bank, recorded as a Sell/Maturity
// transaction. Reading P/L off that 0 would render a misleading −100%
// against cost, so we suppress the P/L line for sold *and* matured
// positions and surface "Sold on {date}" / "Matured on {date}" instead
// (presentation interpreting true data). This re-widens the branch that
// #17 had narrowed to sold-only back when Maturity wrote a fictional
// principal+interest close snapshot — #25 removed that false row.

import type { TFunction } from "i18next";
import { formatCurrency, formatDate, formatSignedPercent } from "@/lib/format";
import { cn } from "@/lib/utils";
import { ledgerTotals, type LedgerTotalKey } from "@/lib/ledgerTotals";
import type { InvestmentSubtype } from "@/lib/lifecycle";
import type { InfoField } from "@/components/detail/types";
import type { InvestmentTransaction } from "@/api/types";

// The ledger total each subtype shows as its "received" row. Gold earns none.
const INCOME_KEY: Partial<Record<InvestmentSubtype, LedgerTotalKey>> = {
  bond: "coupon",
  stock: "dividend",
  mutual_fund: "distribution",
  time_deposit: "maturityInterest",
};

type Input = {
  currency: string;
  // Latest snapshot's amount (already in native currency). Null when
  // there are no snapshots yet.
  latestValue: number | null;
  // Cost basis as of "now" — caller computes via lib/costBasis based on
  // subtype quirks (ledger replay for stock/MF/gold/bond; flat principal for
  // time deposit — bonds always carry a Buy at placement now, issue #27).
  totalCost: number;
  // When set to a terminal status ('sold' | 'matured') with a
  // terminated_at, swaps the P/L row for "Sold on {date}" / "Matured on
  // {date}". Pass `investment.status` + `investment.terminated_at`.
  status?: string | null;
  terminatedAt?: string | null;
  subtype: InvestmentSubtype;
  // The position's ledger; the income row totals it (lib/ledgerTotals).
  transactions: InvestmentTransaction[];
};

export function investmentHeadlineFields(
  { currency, latestValue, totalCost, status, terminatedAt, subtype, transactions }: Input,
  t: TFunction,
): InfoField[] {
  const isClosed = !!((status === "sold" || status === "matured") && terminatedAt);
  // P/L is meaningful only when we have a current value to compare cost
  // against. No snapshot → no P/L number to show.
  const pl = latestValue !== null ? latestValue - totalCost : null;
  const plPct = pl !== null && Math.abs(totalCost) > 0 ? (pl / totalCost) * 100 : null;

  // A time deposit's interest arrives only at maturity, so its row would read 0
  // for the whole term; it appears once the deposit has matured (#994).
  const incomeKey =
    subtype === "time_deposit" && status !== "matured" ? undefined : INCOME_KEY[subtype];
  const income = incomeKey
    ? ledgerTotals(subtype, transactions, currency).find((total) => total.key === incomeKey)
    : undefined;
  const plHint = [
    t("investments:headline.unrealizedPLHint"),
    incomeKey ? t(`investments:headline.unrealizedPLExcludes.${incomeKey}`) : null,
  ]
    .filter(Boolean)
    .join(" ");

  return [
    {
      label: t("investments:headline.totalValue"),
      value:
        latestValue === null ? (
          <span className="text-muted-foreground">{t("investments:headline.totalValueEmpty")}</span>
        ) : (
          <span data-testid="investment-headline-value">
            {formatCurrency(latestValue.toString(), currency)}
          </span>
        ),
    },
    {
      label: t("investments:headline.totalCost"),
      value: <span>{formatCurrency(totalCost.toString(), currency)}</span>,
    },
    isClosed
      ? {
          label: t(
            status === "matured"
              ? "investments:headline.closed.matured"
              : "investments:headline.closed.sold",
          ),
          value: <span data-testid="investment-headline-closed">{formatDate(terminatedAt)}</span>,
        }
      : {
          label: t("investments:headline.unrealizedPL"),
          hint: plHint,
          value:
            pl === null ? (
              <span className="text-muted-foreground">
                {t("investments:headline.unrealizedPLEmpty")}
              </span>
            ) : (
              <span className={cn("", plColor(pl))} data-testid="investment-headline-pl">
                {formatPL(pl, plPct, currency)}
              </span>
            ),
        },
    ...(income && incomeKey
      ? [
          {
            label: t(`investments:headline.income.${incomeKey}`),
            value: (
              <span data-testid="investment-headline-income">
                {income.amounts.map((a) => formatCurrency(a.amount, a.currency)).join(" · ")}
              </span>
            ),
          },
        ]
      : []),
  ];
}

function plColor(pl: number): string {
  if (pl > 0) return "text-positive";
  if (pl < 0) return "text-negative";
  return "text-muted-foreground";
}

// `+Rp 1,234,567 (+2.34%)` / `−Rp 234,567 (−1.50%)` / `+Rp 100` (when
// cost is zero and percentage can't be computed). Signs use the minus
// glyph "−" U+2212 (not hyphen) for the same reason the revaluation
// helper does — it visually aligns with "+" on inline text.
function formatPL(pl: number, plPct: number | null, currency: string): string {
  const sign = pl > 0 ? "+" : pl < 0 ? "−" : "";
  const amount = `${sign}${formatCurrency(Math.abs(pl).toString(), currency)}`;
  if (plPct === null) return amount;
  return `${amount} (${formatSignedPercent(plPct)})`;
}

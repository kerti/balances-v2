// The investment detail card's money summary, as label/value fields for the
// card's one InfoGrid (#839; it used to be its own stat column, ADR-0051 Phase B).
// Top to bottom: total cost, P/L, total value — then any type-specific
// descriptive rows the descriptor threads in (`extraFields` — bond coupon /
// disposition, time-deposit Period + at-maturity). Risk profile is not here — it
// rides the details-card header as a compact badge (via the descriptor's
// `renderHeaderBadge` slot).
//
// Total value (the latest snapshot's amount) sits next to cost + P/L — the three
// read together as the position's money summary.
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
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { InfoField } from "@/components/detail/types";

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
  // Type-specific descriptive rows the descriptor threads in below the money
  // stats. Empty for the qty×price types.
  extraFields?: InfoField[];
};

export function investmentHeadlineFields(
  { currency, latestValue, totalCost, status, terminatedAt, extraFields }: Input,
  t: TFunction,
): InfoField[] {
  const isClosed = !!((status === "sold" || status === "matured") && terminatedAt);
  // P/L is meaningful only when we have a current value to compare cost
  // against. No snapshot → no P/L number to show.
  const pl = latestValue !== null ? latestValue - totalCost : null;
  const plPct = pl !== null && Math.abs(totalCost) > 0 ? (pl / totalCost) * 100 : null;

  return [
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
          hint: t("investments:headline.unrealizedPLHint"),
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
    ...(extraFields ?? []),
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
  const pctSign = plPct > 0 ? "+" : plPct < 0 ? "−" : "";
  return `${amount} (${pctSign}${Math.abs(plPct).toFixed(2)}%)`;
}

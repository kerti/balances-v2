// Shared cost / P/L stat row rendered just below the H1 subtitle on each
// investment detail screen (issue #14, slice 14b). Sits as a single
// horizontal flex row so the page header stays compact.
//
// "Latest value" is deliberately omitted — it's already prominent in the
// snapshots card; repeating it here would clutter the headline. The
// numbers shown are the two new-to-the-user signals: how much they put
// in, and whether they're up or down.
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

import { useTranslation } from "react-i18next";
import { formatCurrency, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";

type Props = {
  currency: string;
  // Latest snapshot's amount (already in native currency). Null when
  // there are no snapshots yet.
  latestValue: number | null;
  // Cost basis as of "now" — caller computes via lib/costBasis based on
  // subtype quirks (ledger replay for stock/MF/gold/bond; flat principal for
  // time deposit — bonds always carry a Buy at placement now, issue #27).
  totalCost: number;
  // When set to a terminal status ('sold' | 'matured') with a
  // terminated_at, swaps the P/L block for "Sold on {date}" / "Matured on
  // {date}". Pass `investment.status` + `investment.terminated_at`.
  status?: string | null;
  terminatedAt?: string | null;
};

export function InvestmentHeadline({
  currency,
  latestValue,
  totalCost,
  status,
  terminatedAt,
}: Props) {
  const { t } = useTranslation("investments");

  const isClosed = !!(
    (status === "sold" || status === "matured") &&
    terminatedAt
  );
  // P/L is meaningful only when we have a current value to compare cost
  // against. No snapshot → no P/L number to show.
  const pl = latestValue !== null ? latestValue - totalCost : null;
  const plPct =
    pl !== null && Math.abs(totalCost) > 0 ? (pl / totalCost) * 100 : null;

  return (
    <div
      className="mt-2 flex flex-wrap gap-x-6 gap-y-1 text-sm"
      data-testid="investment-headline"
    >
      <div>
        <span className="text-muted-foreground">{t("headline.totalCost")}</span>{" "}
        <span className="tabular-nums">
          {formatCurrency(totalCost.toString(), currency)}
        </span>
      </div>
      {isClosed ? (
        <div data-testid="investment-headline-closed">
          <span className="text-muted-foreground">
            {t(
              status === "matured"
                ? "headline.closed.matured"
                : "headline.closed.sold",
            )}
          </span>{" "}
          <span>{formatDate(terminatedAt)}</span>
        </div>
      ) : (
        <div>
          <span className="text-muted-foreground">
            {t("headline.unrealizedPL")}
          </span>{" "}
          {pl === null ? (
            <span className="text-muted-foreground">
              {t("headline.unrealizedPLEmpty")}
            </span>
          ) : (
            <span
              className={cn("tabular-nums", plColor(pl))}
              data-testid="investment-headline-pl"
            >
              {formatPL(pl, plPct, currency)}
            </span>
          )}
        </div>
      )}
    </div>
  );
}

function plColor(pl: number): string {
  if (pl > 0) return "text-emerald-600";
  if (pl < 0) return "text-destructive";
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

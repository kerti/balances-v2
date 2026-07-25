// Shared cost / P/L / value stat column for the investment detail card's middle
// column (issue #14, slice 14b; relocated + widened in ADR-0051 Phase B). Reads
// as three inline label/value rows — total cost, P/L, total value — via the
// shared InfoGrid `inline` idiom, so each stat's label and figure sit on one
// line at every width. The core places it as the middle column on desktop and
// between the identity + info fields on mobile; it no longer rides under the H1.
//
// Total value (the latest snapshot's amount) is shown here now that the block
// lives in the card next to cost + P/L — the three read together as the
// position's money summary. It was previously omitted from the under-H1 row to
// keep that header compact.
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
import { InfoGrid } from "@/components/detail/InfoGrid";
import type { InfoField } from "@/components/detail/types";

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

  const isClosed = !!((status === "sold" || status === "matured") && terminatedAt);
  // P/L is meaningful only when we have a current value to compare cost
  // against. No snapshot → no P/L number to show.
  const pl = latestValue !== null ? latestValue - totalCost : null;
  const plPct = pl !== null && Math.abs(totalCost) > 0 ? (pl / totalCost) * 100 : null;

  // Each stat reads as an inline label/value row (label left, value right) at
  // every width — the shared InfoGrid `inline` idiom, the same one the details
  // card's ownership meta uses — so the middle column stays a compact money
  // summary rather than a tall stacked list. `ml-auto` on each value node pushes
  // the figures to the column's right edge (the ADR-0051 rule that alignment
  // rides on the value node), so the numbers align right on desktop too — where
  // InfoGrid otherwise left-aligns the value column.
  const fields: InfoField[] = [
    {
      label: t("headline.totalCost"),
      value: (
        <span className="ml-auto tabular-nums">
          {formatCurrency(totalCost.toString(), currency)}
        </span>
      ),
    },
    isClosed
      ? {
          label: t(status === "matured" ? "headline.closed.matured" : "headline.closed.sold"),
          value: (
            <span className="ml-auto" data-testid="investment-headline-closed">
              {formatDate(terminatedAt)}
            </span>
          ),
        }
      : {
          label: t("headline.unrealizedPL"),
          value:
            pl === null ? (
              <span className="ml-auto text-muted-foreground">
                {t("headline.unrealizedPLEmpty")}
              </span>
            ) : (
              <span
                className={cn("ml-auto tabular-nums", plColor(pl))}
                data-testid="investment-headline-pl"
              >
                {formatPL(pl, plPct, currency)}
              </span>
            ),
        },
    {
      label: t("headline.totalValue"),
      value:
        latestValue === null ? (
          <span className="ml-auto text-muted-foreground">{t("headline.totalValueEmpty")}</span>
        ) : (
          <span className="ml-auto tabular-nums" data-testid="investment-headline-value">
            {formatCurrency(latestValue.toString(), currency)}
          </span>
        ),
    },
  ];

  return (
    <div data-testid="investment-headline">
      <InfoGrid fields={fields} mobileLayout="inline" />
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

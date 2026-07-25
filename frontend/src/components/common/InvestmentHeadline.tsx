// Shared stat column for the investment detail card's middle column (issue #14,
// slice 14b; relocated + widened in ADR-0051 Phase B). Reads as inline
// label/value rows via the shared InfoGrid `inline` idiom, so each stat's label
// and figure sit on one line at every width. Top to bottom: the risk profile,
// any type-specific descriptive rows the descriptor threads in (`extraFields` —
// bond coupon/disposition, time-deposit placement/maturity), then the money
// summary — total cost, P/L, total value. Only the money figures right-align.
// The core places it as the middle column on desktop; on mobile the details
// card's three columns stack in reading order, so this block follows the
// identity + info fields. It no longer rides under the H1.
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
import type { RiskProfile } from "@/api/types";

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
  // The position's risk profile — rendered as the first row of the middle
  // column (`investment.risk_profile`, shared by all five investment types).
  riskProfile?: RiskProfile;
  // Type-specific descriptive rows the descriptor threads into the middle
  // column above the money stats (bond: coupon + disposition; time deposit:
  // placement/maturity/at-maturity). Left-aligned — only the money figures
  // right-align. Empty for the qty×price types.
  extraFields?: InfoField[];
};

export function InvestmentHeadline({
  currency,
  latestValue,
  totalCost,
  status,
  terminatedAt,
  riskProfile,
  extraFields,
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
  // summary rather than a tall stacked list. InfoGrid right-aligns the value
  // cell itself, so the value nodes carry only `tabular-nums` / colour, no
  // per-node `ml-auto`.
  const fields: InfoField[] = [
    {
      label: t("headline.totalCost"),
      value: <span className="tabular-nums">{formatCurrency(totalCost.toString(), currency)}</span>,
    },
    isClosed
      ? {
          label: t(status === "matured" ? "headline.closed.matured" : "headline.closed.sold"),
          value: <span data-testid="investment-headline-closed">{formatDate(terminatedAt)}</span>,
        }
      : {
          label: t("headline.unrealizedPL"),
          value:
            pl === null ? (
              <span className="text-muted-foreground">{t("headline.unrealizedPLEmpty")}</span>
            ) : (
              <span
                className={cn("tabular-nums", plColor(pl))}
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
          <span className="text-muted-foreground">{t("headline.totalValueEmpty")}</span>
        ) : (
          <span className="tabular-nums" data-testid="investment-headline-value">
            {formatCurrency(latestValue.toString(), currency)}
          </span>
        ),
    },
    ...(riskProfile
      ? [
          {
            label: t("riskProfile.selectLabel"),
            // e.g. `riskProfile.selectMedium` → "Medium".
            value: t(
              `riskProfile.select${riskProfile.charAt(0).toUpperCase()}${riskProfile.slice(1)}`,
            ),
          },
        ]
      : []),
    ...(extraFields ?? []),
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

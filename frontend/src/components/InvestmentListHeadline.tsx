// Investment-list-specific headline (issue #14, slice 14c). Sibling to
// `ListHeadline` — non-investment list screens keep using ListHeadline
// (which has a single value figure); investment screens swap to this
// one because they also need to surface cost + unrealized P/L.
//
// Layout: one card, three lines (Value / Cost / P/L) with a per-currency
// dot separator on each line. Single-currency household sees one figure
// per line; mixed sees "Rp X · $ Y". Active count under the rows.
//
// P/L tone follows the detail-screen `InvestmentHeadline`: emerald gain
// / destructive loss / muted zero, "−" U+2212 minus glyph.

import { useTranslation } from "react-i18next";
import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { CurrencyAggregate } from "@/lib/listAggregates";

type Props = {
  aggregates: CurrencyAggregate[];
  count: number;
  noun: string;
  nounPlural: string;
  testId?: string;
};

export function InvestmentListHeadline({ aggregates, count, noun, nounPlural, testId }: Props) {
  const { t } = useTranslation(["common", "investments"]);
  if (aggregates.length === 0) return null;
  return (
    <div className="rounded-lg border p-4" data-testid={testId}>
      <div className="text-sm text-muted-foreground">{t("investments:list.totalValue")}</div>
      <div className="mt-0.5 text-2xl font-semibold tabular-nums">
        {aggregates.map((a, i) => (
          <span key={a.currency}>
            {i > 0 && <Sep />}
            {formatCurrency(String(a.value), a.currency)}
          </span>
        ))}
      </div>
      <div className="mt-1 text-sm text-muted-foreground tabular-nums">
        <span>{t("investments:list.totalCost")}</span>{" "}
        {aggregates.map((a, i) => (
          <span key={a.currency}>
            {i > 0 && <Sep />}
            {formatCurrency(String(a.cost), a.currency)}
          </span>
        ))}
      </div>
      <div className="mt-0.5 text-sm">
        <span className="text-muted-foreground">{t("investments:list.unrealizedPL")}</span>{" "}
        {aggregates.map((a, i) => (
          <span key={a.currency}>
            {i > 0 && <SepMuted />}
            <span className={cn("tabular-nums", plColor(a.pl))}>{formatPL(a)}</span>
          </span>
        ))}
      </div>
      <div className="mt-1 text-xs text-muted-foreground">
        {t("common:list.activeCount", {
          count,
          noun: count === 1 ? noun : nounPlural,
        })}
      </div>
    </div>
  );
}

function Sep() {
  return (
    <span aria-hidden className="text-muted-foreground">
      {" · "}
    </span>
  );
}

function SepMuted() {
  return (
    <span aria-hidden className="text-muted-foreground">
      {" · "}
    </span>
  );
}

function plColor(pl: number): string {
  if (pl > 0) return "text-emerald-600";
  if (pl < 0) return "text-destructive";
  return "text-muted-foreground";
}

function formatPL({ cost, pl, currency }: CurrencyAggregate): string {
  const sign = pl > 0 ? "+" : pl < 0 ? "−" : "";
  const amount = `${sign}${formatCurrency(Math.abs(pl).toString(), currency)}`;
  if (Math.abs(cost) === 0) return amount;
  const pct = (pl / cost) * 100;
  const pctSign = pct > 0 ? "+" : pct < 0 ? "−" : "";
  return `${amount} (${pctSign}${Math.abs(pct).toFixed(2)}%)`;
}

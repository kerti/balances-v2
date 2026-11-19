// Investment-list-specific headline (issue #14, slice 14c). Sibling to
// `ListHeadline` — non-investment list screens keep using ListHeadline
// (which has a single value figure); investment screens swap to this
// one because they also need to surface cost + unrealized P/L.
//
// With `converted`, the Value line is one reporting-currency figure (plus a muted
// native-subtotals line) and Cost / P/L stay per currency underneath (#937).
//
// Layout: one card, three lines (Value / Cost / P/L) with a per-currency
// dot separator on each line. Single-currency household sees one figure
// per line; mixed sees "Rp X · $ Y". Active count under the rows.
//
// P/L tone follows the detail-screen `investmentHeadlineFields`: emerald gain
// / destructive loss / muted zero, "−" U+2212 minus glyph.

import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { formatCurrency, formatSignedPercent } from "@/lib/format";
import { cn } from "@/lib/utils";
import { headlineSurface } from "@/lib/headline";
import { InfoHint } from "@/components/common/InfoHint";
import type { CurrencyAggregate } from "@/lib/listAggregates";

type Props = {
  aggregates: CurrencyAggregate[];
  count: number;
  noun: string;
  nounPlural: string;
  testId?: string;
  // The value in the reporting currency from the engine (#937). When set it
  // replaces the per-currency value figures; cost and P/L stay per currency.
  converted?: { total: string; currency: string };
  // Rendered beneath the count (the missing-rate warning).
  children?: ReactNode;
};

export function InvestmentListHeadline({
  aggregates,
  count,
  noun,
  nounPlural,
  testId,
  converted,
  children,
}: Props) {
  const { t } = useTranslation(["common", "investments"]);
  if (aggregates.length === 0) return null;
  // Multi-currency stacks one figure per line on mobile (ADR-0050) — value,
  // cost, and P/L each drop their additional currencies below the primary
  // instead of running off to its right. Single-currency renders inline
  // unchanged (the labelled cost/PL rows stay label + figure on one line).
  const multi = aggregates.length > 1;
  const stack = multi ? "block md:inline" : undefined;
  const nativeValues = aggregates.filter((a) => a.currency !== converted?.currency);
  return (
    <div className={cn("rounded-lg border p-4", headlineSurface)} data-testid={testId}>
      <div className="text-sm text-muted-foreground">{t("investments:list.totalValue")}</div>
      {converted ? (
        <>
          <div className="mt-0.5 text-2xl font-semibold" data-testid="home-total-value">
            {formatCurrency(converted.total, converted.currency)}
          </div>
          {nativeValues.length > 0 && (
            <div className="mt-0.5 text-sm text-muted-foreground" data-testid="home-total-native">
              {t("common:hub.nativeSubtotals", {
                amounts: nativeValues
                  .map((a) => formatCurrency(String(a.value), a.currency))
                  .join(" · "),
              })}
            </div>
          )}
        </>
      ) : (
        <div className="mt-0.5 text-2xl font-semibold">
          {aggregates.map((a, i) => (
            <span key={a.currency} className={stack}>
              {i > 0 && <Sep />}
              {formatCurrency(String(a.value), a.currency)}
            </span>
          ))}
        </div>
      )}
      <div className="mt-1 text-sm text-muted-foreground">
        <span>{t("investments:list.totalCost")}</span>{" "}
        {aggregates.map((a, i) => (
          <span key={a.currency} className={stack}>
            {i > 0 && <Sep />}
            {formatCurrency(String(a.cost), a.currency)}
          </span>
        ))}
      </div>
      <div className="mt-0.5 text-sm">
        <span className="text-muted-foreground">{t("investments:list.unrealizedPL")}</span>
        <InfoHint
          label={t("investments:list.unrealizedPL")}
          testId="list-gain-loss-hint"
          className="-my-3 ml-0.5"
        >
          {t("investments:headline.unrealizedPLHint")}
        </InfoHint>{" "}
        {aggregates.map((a, i) => (
          <span key={a.currency} className={stack}>
            {i > 0 && <SepMuted />}
            <span className={cn("", plColor(a.pl))}>{formatPL(a)}</span>
          </span>
        ))}
      </div>
      <div className="mt-1 text-xs text-muted-foreground">
        {t("common:list.activeCount", {
          count,
          noun: count === 1 ? noun : nounPlural,
        })}
      </div>
      {children && <div className="mt-2">{children}</div>}
    </div>
  );
}

// Inline dot separator — hidden on mobile where the per-currency spans stack
// onto their own lines (ADR-0050), shown on md+ where figures run inline.
function Sep() {
  return (
    <span aria-hidden className="hidden text-muted-foreground md:inline">
      {" · "}
    </span>
  );
}

function SepMuted() {
  return (
    <span aria-hidden className="hidden text-muted-foreground md:inline">
      {" · "}
    </span>
  );
}

function plColor(pl: number): string {
  if (pl > 0) return "text-positive";
  if (pl < 0) return "text-negative";
  return "text-muted-foreground";
}

function formatPL({ cost, pl, currency }: CurrencyAggregate): string {
  const sign = pl > 0 ? "+" : pl < 0 ? "−" : "";
  const amount = `${sign}${formatCurrency(Math.abs(pl).toString(), currency)}`;
  if (Math.abs(cost) === 0) return amount;
  return `${amount} (${formatSignedPercent((pl / cost) * 100)})`;
}

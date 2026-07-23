import { useTranslation } from "react-i18next";
import { formatCurrency } from "@/lib/format";
import { headlineSurface } from "@/lib/headline";
import { cn } from "@/lib/utils";
import type { CurrencyTotal } from "@/lib/totals";

type Props = {
  totals: CurrencyTotal[];
  count: number;
  // e.g. "Total balance" / "Total value" / "Total owed".
  label: string;
  // Singular + plural for the count line ("1 account" / "3 accounts"); explicit
  // because plurals are irregular (property → properties). Indonesian collapses
  // both forms to the same noun — same prop shape works for either locale.
  noun: string;
  nounPlural: string;
  testId?: string;
};

// The per-currency active total shown above a list screen's table. Currencies
// stay separate (no FX — see lib/totals); a single-currency household sees one
// figure, a mixed one sees "Rp … · $ …". Renders nothing when no active
// position carries a balance.
export function ListHeadline({ totals, count, label, noun, nounPlural, testId }: Props) {
  const { t } = useTranslation("common");
  if (totals.length === 0) return null;
  // Multi-currency stacks one figure per line on mobile (ADR-0050) — additional
  // currencies drop below the primary instead of running off to its right;
  // single-currency renders inline unchanged.
  const multi = totals.length > 1;
  return (
    <div className={cn("rounded-lg border p-4", headlineSurface)} data-testid={testId}>
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-2xl font-semibold tabular-nums">
        {totals.map((row, i) => (
          <span key={row.currency} className={multi ? "block md:inline" : undefined}>
            {i > 0 && (
              <span aria-hidden className="hidden text-muted-foreground md:inline">
                {/* Typographic separator glyph; locale-neutral. */}
                {" · "}
              </span>
            )}
            {formatCurrency(String(row.amount), row.currency)}
          </span>
        ))}
      </div>
      <div className="mt-0.5 text-xs text-muted-foreground">
        {t("list.activeCount", {
          count,
          noun: count === 1 ? noun : nounPlural,
        })}
      </div>
    </div>
  );
}

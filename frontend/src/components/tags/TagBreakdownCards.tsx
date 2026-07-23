import { useTranslation } from "react-i18next";
import { TagBadge } from "@/components/common/TagBadge";
import { cellKey, type CurrencyBreakdown } from "@/lib/tagBreakdown";
import { formatCurrency } from "@/lib/format";

type Props = {
  bd: CurrencyBreakdown;
  isChecked: (key: string) => boolean;
  toggle: (key: string) => void;
};

// Mobile leaf renderer (ADR-0050 "wide table → stacked cards"): one card per
// tag, the net value promoted to the card headline so the number the household
// member came for reads with no horizontal scroll, holdings and liabilities
// stacked as label→value pairs below. The pie-inclusion checkbox and tag badge
// share the top line; the whole line is a ≥44px tap target (a <label> at the
// mobile a11y floor) rather than the desktop table's bare 16px checkbox. Keyed
// by cellKey so it toggles the same slice the desktop table does. A distinct,
// checkbox-less Total card closes the stack.
export function TagBreakdownCards({ bd, isChecked, toggle }: Props) {
  const { t } = useTranslation(["tags"]);

  return (
    <div className="space-y-2" data-testid={`tag-breakdown-cards-${bd.currency}`}>
      {bd.cells.map((c) => {
        const key = cellKey(c);
        const on = isChecked(key);
        return (
          <div key={key} className="rounded-lg border p-3">
            <div className="flex items-center gap-3">
              {/* Top line doubles as the tap target: label wraps the checkbox +
                  badge and clears the 44px floor via min-h-11. */}
              <label className="flex min-h-11 flex-1 cursor-pointer items-center gap-2">
                <input
                  type="checkbox"
                  checked={on}
                  onChange={() => toggle(key)}
                  className="h-4 w-4 shrink-0 cursor-pointer accent-primary"
                  aria-label={c.name}
                />
                <TagBadge name={c.name} color={c.color} />
              </label>
              <div
                className="shrink-0 text-right text-lg font-semibold tabular-nums"
                data-testid="tag-breakdown-net"
              >
                {formatCurrency(String(c.net), bd.currency)}
              </div>
            </div>
            <dl className="mt-1 space-y-1 text-sm">
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t("report.col.holdings")}</dt>
                <dd className="tabular-nums">
                  {c.holdings > 0 ? formatCurrency(String(c.holdings), bd.currency) : "—"}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t("report.col.liabilities")}</dt>
                <dd className="tabular-nums text-destructive">
                  {c.liabilities > 0
                    ? `−${formatCurrency(String(c.liabilities), bd.currency)}`
                    : "—"}
                </dd>
              </div>
            </dl>
          </div>
        );
      })}

      {/* Total: no checkbox — it's a summary, not a toggleable slice. */}
      <div className="rounded-lg border bg-muted/50 p-3 font-medium">
        <div className="flex items-center justify-between gap-3">
          <span>{t("report.total")}</span>
          <span className="text-right text-lg tabular-nums">
            {formatCurrency(String(bd.totalHoldings - bd.totalLiabilities), bd.currency)}
          </span>
        </div>
        <dl className="mt-1 space-y-1 text-sm">
          <div className="flex justify-between gap-3">
            <dt className="text-muted-foreground">{t("report.col.holdings")}</dt>
            <dd className="tabular-nums">
              {formatCurrency(String(bd.totalHoldings), bd.currency)}
            </dd>
          </div>
          <div className="flex justify-between gap-3">
            <dt className="text-muted-foreground">{t("report.col.liabilities")}</dt>
            <dd className="tabular-nums text-destructive">
              {bd.totalLiabilities > 0
                ? `−${formatCurrency(String(bd.totalLiabilities), bd.currency)}`
                : "—"}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  );
}

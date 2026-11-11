// Headline of an overview hub that renders in the reporting currency (#845):
// the converted total, a muted line of the non-reporting native subtotals
// ("incl. USD 12,345 · converted at each month's rate"), the active-position
// count, and — as children — the missing-rate warning. The converted `total` arrives
// from the backend as a decimal string and is formatted as-is.

import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { formatCurrency } from "@/lib/format";
import { headlineSurface } from "@/lib/headline";
import { cn } from "@/lib/utils";

type Props = {
  label: string;
  total: string;
  currency: string;
  nativeSubtotals: { currency: string; value: number }[];
  countText: string;
  testId: string;
  children?: ReactNode;
};

export function ConvertedHeadline({
  label,
  total,
  currency,
  nativeSubtotals,
  countText,
  testId,
  children,
}: Props) {
  const { t } = useTranslation("common");
  return (
    <div className={cn("rounded-lg border p-4", headlineSurface)} data-testid={testId}>
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-2xl font-semibold" data-testid="home-total-value">
        {formatCurrency(total, currency)}
      </div>
      {nativeSubtotals.length > 0 && (
        <div className="mt-0.5 text-sm text-muted-foreground" data-testid="home-total-native">
          {t("hub.nativeSubtotals", {
            amounts: nativeSubtotals
              .map((s) => formatCurrency(String(s.value), s.currency))
              .join(" · "),
          })}
        </div>
      )}
      <div className="mt-1 text-xs text-muted-foreground">{countText}</div>
      {children && <div className="mt-2">{children}</div>}
    </div>
  );
}

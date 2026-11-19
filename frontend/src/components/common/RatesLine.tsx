import { useTranslation } from "react-i18next";
import { formatNumber, formatShortYearMonth } from "@/lib/format";
import { resolveDisplayRate } from "@/lib/fx";
import type { FxRate } from "@/api/types";

// RatesLine is the one muted line under a converted headline: the rates applied
// this month (fx_rates_used). A rate carried forward from an earlier month says
// so on the same line, so the rate and its caveat can't disagree. Shared by the
// dashboard and the Tags report; renders nothing when no rate was used.
export function RatesLine({
  fxRatesUsed,
  yearMonth,
  currency,
  rates,
  testIdPrefix = "dashboard",
}: {
  fxRatesUsed: Record<string, string>;
  yearMonth: string;
  currency: string;
  rates: FxRate[];
  testIdPrefix?: string;
}) {
  const { t } = useTranslation("dashboard");
  const entries = Object.entries(fxRatesUsed).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) return null;
  const month = yearMonth.slice(0, 7);
  return (
    <p data-testid={`${testIdPrefix}-rates`} className="text-sm text-muted-foreground">
      {t("rates.prefix")}{" "}
      {entries.map(([cur, rate], i) => {
        const from = resolveDisplayRate(rates, cur, yearMonth)?.rateMonth;
        const carried = from !== undefined && from.slice(0, 7) !== month;
        return (
          <span key={cur} data-testid={`${testIdPrefix}-rate-${cur}`}>
            {i > 0 && " · "}
            {t("rates.line", { base: cur, rate: formatNumber(rate), quote: currency })}
            {carried && (
              <span data-testid={`${testIdPrefix}-rate-carried-${cur}`} className="text-warning">
                {" · "}
                {t("rates.carriedForward", {
                  from: formatShortYearMonth(
                    new Date(Number(from.slice(0, 4)), Number(from.slice(5, 7)) - 1, 1),
                  ),
                })}
              </span>
            )}
          </span>
        );
      })}
    </p>
  );
}

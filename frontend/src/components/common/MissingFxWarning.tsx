import { useTranslation } from "react-i18next";
import { formatYearMonth } from "@/lib/format";
import type { MissingFx } from "@/api/types";

// The ADR-0002 "exclude and warn" line, shared by the dashboard headline (a net
// worth figure) and the overview hubs (one group's total) so a missing exchange
// rate reads the same everywhere. `scope` only swaps the opening noun.
export function MissingFxWarning({
  missingFx,
  yearMonth,
  scope = "netWorth",
}: {
  missingFx: MissingFx[];
  yearMonth: string;
  scope?: "netWorth" | "groupTotal";
}) {
  const { t } = useTranslation("dashboard");
  if (missingFx.length === 0) return null;
  const currencies = [...new Set(missingFx.map((m) => m.currency))];
  const count = missingFx.filter((m) => m.position_id !== null).length;
  const subject = count > 0 ? t("missingFx.positions", { count }) : t("missingFx.someAmounts");
  const addRate = t("missingFx.addRate", { count: currencies.length });
  return (
    <p className="text-sm text-destructive" data-testid="missing-fx-warning">
      {t(scope === "groupTotal" ? "missingFx.lineGroup" : "missingFx.line", {
        subject,
        when: formatYearMonth(yearMonth),
        currencies: currencies.join(", "),
        addRate,
      })}
    </p>
  );
}

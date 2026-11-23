import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { SettingsRow } from "@/components/common/SettingsSurface";
import { useRebuildReports, useReports } from "@/hooks/useReports";
import { formatYearMonth } from "@/lib/format";

// RebuildRow offers the manual report rebuild (ADR-0006) in Settings ▸ Reports,
// framed in user terms rather than the engine-cache reason. Rebuild-all
// re-derives every month (needed after an exchange-rate correction ripples
// across history). Rebuild-month is the surgical fix; Settings has no month
// picker, so it targets the latest month — the one the dashboard opens on.
export function RebuildRow() {
  const { t } = useTranslation("settings");
  const { data: reports } = useReports();
  const { rebuildAll, rebuildMonth } = useRebuildReports();
  const busy = rebuildAll.isPending || rebuildMonth.isPending;
  const latest = reports && reports.length > 0 ? reports[reports.length - 1] : null;

  return (
    <SettingsRow name={t("reports.rebuild.title")} description={t("reports.rebuild.description")}>
      <div className="flex flex-col gap-2">
        {latest && (
          <Button
            variant="outline"
            data-testid="rebuild-month"
            disabled={busy}
            onClick={() => rebuildMonth.mutate(latest.year_month)}
          >
            {rebuildMonth.isPending
              ? t("reports.rebuild.rebuilding")
              : t("reports.rebuild.rebuildMonth", { when: formatYearMonth(latest.year_month) })}
          </Button>
        )}
        <Button
          variant="outline"
          data-testid="rebuild-all"
          disabled={busy}
          onClick={() => rebuildAll.mutate()}
        >
          {rebuildAll.isPending ? t("reports.rebuild.rebuilding") : t("reports.rebuild.rebuildAll")}
        </Button>
      </div>
    </SettingsRow>
  );
}

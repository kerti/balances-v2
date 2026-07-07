import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import i18n from "@/i18n";
import { importWithReloadGuard } from "@/lib/lazyWithReload";
import { triggerDownload } from "@/lib/backup";
import type { FxRate, HouseholdMember, MonthlyReport } from "@/api/types";
import type { Me } from "@/hooks/useSession";

type Props = {
  reports: MonthlyReport[];
  selected: MonthlyReport;
  currency: string;
  secondaryCurrency: string;
  rates: FxRate[];
  members: HouseholdMember[] | undefined;
  me: Me | null | undefined;
};

// The button itself is always rendered — it's tiny. @react-pdf/renderer
// (~1.4MB, far larger than any other lazy chunk in this app) is fetched only
// on click, not eagerly on mount: unlike SnapshotChart's small recharts chunk
// (needed immediately for the page's core content), this is a click-triggered,
// comparatively rare action, so eagerly loading it on every dashboard view is
// bloat disproportionate to how often it's used (ADR-0044). Reuses
// importWithReloadGuard directly (the same post-deploy chunk-reload recovery
// lazyWithReload wraps for React.lazy) since this is a plain dynamic import,
// not a lazily-mounted component.
export function ReportPdfButton({
  reports,
  selected,
  currency,
  secondaryCurrency,
  rates,
  members,
  me,
}: Props) {
  const { t } = useTranslation("dashboard");
  const [busy, setBusy] = useState(false);

  async function handleClick() {
    setBusy(true);
    try {
      const [{ pdf }, { buildReportPdfData }, { ReportDocument }] = await Promise.all([
        importWithReloadGuard(() => import("@react-pdf/renderer")),
        importWithReloadGuard(() => import("@/lib/pdf/reportPdfData")),
        importWithReloadGuard(() => import("@/lib/pdf/ReportDocument")),
      ]);
      const data = buildReportPdfData({ reports, selected, currency, secondaryCurrency, rates });
      const fixedT = i18n.getFixedT(i18n.language, "dashboard");
      const blob = await pdf(
        <ReportDocument data={data} t={fixedT} members={members} me={me} />,
      ).toBlob();
      triggerDownload(blob, `Balances_${data.yearMonth.slice(0, 7)}.pdf`);
      toast.success(t("downloadPdf.exported"));
    } catch {
      toast.error(t("downloadPdf.failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Button
      variant="outline"
      onClick={() => void handleClick()}
      disabled={busy}
      data-testid="download-pdf-button"
    >
      {busy ? t("downloadPdf.preparing") : t("downloadPdf.button")}
    </Button>
  );
}

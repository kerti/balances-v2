import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { pdf } from "@react-pdf/renderer";
import { Button } from "@/components/ui/button";
import i18n from "@/i18n";
import { triggerDownload } from "@/lib/backup";
import { buildReportPdfData } from "@/lib/pdf/reportPdfData";
import { ReportDocument } from "@/lib/pdf/ReportDocument";
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

// The heavy half of the lazy boundary (see ReportPdfButton.tsx) — this is
// where @react-pdf/renderer actually loads. i18n.getFixedT (not
// useTranslation's hook binding) is what ReportDocument needs: it renders
// outside the app's React tree via pdf().toBlob(), a separate reconciler with
// no I18nextProvider context (ADR-0044).
export default function ReportPdfButtonImpl({
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

import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { triggerDownload } from "@/lib/backup";
import type { MonthlyReport } from "@/api/types";

type Props = {
  selected: MonthlyReport;
};

// The monthly report PDF is rendered server-side (ADR-0045, superseding
// ADR-0044's client-side @react-pdf/renderer path): this button just fetches
// the bytes from GET /api/reports/{yearMonth}/pdf (same-origin, so the session
// cookie travels) and saves them. Locale and currency are resolved server-side
// from the household + the authenticated user's preference — no props needed
// beyond which month to export.
export function ReportPdfButton({ selected }: Props) {
  const { t } = useTranslation("dashboard");
  const [busy, setBusy] = useState(false);

  async function handleClick() {
    setBusy(true);
    try {
      const ym = selected.year_month.slice(0, 7);
      const res = await fetch(`/api/reports/${ym}/pdf`);
      if (!res.ok) throw new Error("pdf export failed");
      const blob = await res.blob();
      triggerDownload(blob, `Balances_${ym}.pdf`);
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

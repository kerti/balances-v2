import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Download, Loader2 } from "lucide-react";
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

  // Icon + the untranslated format name "PDF": a fixed-width button that reads
  // the same in every locale (so it never wraps out of the toolbar the way a
  // translated "Download PDF" / "Unduh PDF" did). The full, translated phrase
  // stays the accessible name (aria-label) and hover title, so screen-reader
  // and tooltip users still get the verb. Busy swaps the download glyph for a
  // spinner — same footprint, no width shift.
  const label = busy ? t("downloadPdf.preparing") : t("downloadPdf.button");
  return (
    <Button
      variant="outline"
      onClick={() => void handleClick()}
      disabled={busy}
      data-testid="download-pdf-button"
      className="h-11 md:h-8"
      aria-label={label}
      title={label}
    >
      {busy ? (
        <Loader2 className="size-4 animate-spin" aria-hidden />
      ) : (
        <Download className="size-4" aria-hidden />
      )}
      {t("downloadPdf.short")}
    </Button>
  );
}

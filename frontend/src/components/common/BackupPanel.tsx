import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { PanelActions, SettingsPanel } from "@/components/common/SettingsSurface";
import { downloadBackup, type Fidelity } from "@/lib/backup";

// BackupPanel is the export half of Settings → Data (ADR-0036, issue #174). The
// fidelity toggle chooses a full (lossless, carries deleted items) or compacted
// (current data only) backup; progress is indeterminate because the size is
// unknown up front. Restore is the sibling panel below it (#175).
export function BackupPanel() {
  const { t } = useTranslation("settings");
  const [fidelity, setFidelity] = useState<Fidelity>("full");
  const [exporting, setExporting] = useState(false);

  const handleExport = async () => {
    setExporting(true);
    try {
      await downloadBackup(fidelity);
      toast.success(t("data.exported"));
    } catch {
      toast.error(t("data.exportError"));
    } finally {
      setExporting(false);
    }
  };

  return (
    <SettingsPanel title={t("data.title")} description={t("data.description")}>
      <fieldset className="space-y-2" disabled={exporting}>
        <legend className="text-sm font-medium">{t("data.fidelity.label")}</legend>
        {(["full", "compacted"] as const).map((opt) => (
          <label key={opt} className="flex max-md:min-h-11 items-start gap-2 text-sm">
            <input
              type="radio"
              name="backup-fidelity"
              className="mt-1 h-4 w-4"
              value={opt}
              checked={fidelity === opt}
              onChange={() => setFidelity(opt)}
              data-testid={`backup-fidelity-${opt}`}
            />
            <span>
              <span className="font-medium">{t(`data.fidelity.${opt}`)}</span>
              <span className="block text-muted-foreground">{t(`data.fidelity.${opt}Hint`)}</span>
            </span>
          </label>
        ))}
      </fieldset>

      <PanelActions note={t("data.largeNote")}>
        <Button
          variant="outline"
          onClick={handleExport}
          disabled={exporting}
          data-testid="backup-export-button"
        >
          {exporting ? t("data.exporting") : t("data.export")}
        </Button>
      </PanelActions>
    </SettingsPanel>
  );
}

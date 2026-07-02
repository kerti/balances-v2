import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { downloadBackup, type Fidelity } from "@/lib/backup";

// BackupCard is the export half of Settings → Data (ADR-0036, issue #174). The
// fidelity toggle chooses a full (lossless, carries deleted items) or compacted
// (current data only) backup; progress is indeterminate because the size is
// unknown up front. Restore arrives in a later slice (#175).
export function BackupCard() {
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
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("data.title")}</CardTitle>
        <CardDescription>{t("data.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <fieldset className="space-y-2" disabled={exporting}>
          <legend className="text-sm font-medium">
            {t("data.fidelity.label")}
          </legend>
          {(["full", "compacted"] as const).map((opt) => (
            <label key={opt} className="flex items-start gap-2 text-sm">
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
                <span className="block text-muted-foreground">
                  {t(`data.fidelity.${opt}Hint`)}
                </span>
              </span>
            </label>
          ))}
        </fieldset>

        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            onClick={handleExport}
            disabled={exporting}
            data-testid="backup-export-button"
          >
            {exporting ? t("data.exporting") : t("data.export")}
          </Button>
          <p className="text-sm text-muted-foreground">{t("data.largeNote")}</p>
        </div>
      </CardContent>
    </Card>
  );
}

import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import type { SwingAsk } from "@/lib/valueSwing";

type Props = {
  // The question to ask, or null while there is none.
  ask: SwingAsk | null;
  onConfirm: () => void;
  onCancel: () => void;
};

// "Is this value right?" before a single-entry Record snapshot saves a figure
// 10x or a tenth of the last one (#933). It is the plain, non-destructive
// ConfirmDialog (ADR-0058 §3's confirm tier, not the red one: nothing is lost,
// the replace is archived and the save's own toast still offers Undo).
export function SwingConfirmDialog({ ask, onConfirm, onCancel }: Props) {
  const { t } = useTranslation("common");
  // Hold the last question while the dialog fades out, so its text doesn't empty.
  const [shown, setShown] = useState(ask);
  if (ask !== null && ask !== shown) setShown(ask);
  return (
    <ConfirmDialog
      open={ask !== null}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
      title={t("snapshot.swing.title")}
      description={
        shown
          ? t(shown.swing === "up" ? "snapshot.swing.up" : "snapshot.swing.down", {
              value: shown.value,
              prior: shown.prior,
              month: shown.month,
            })
          : undefined
      }
      confirmLabel={t("snapshot.swing.confirm")}
      cancelLabel={t("snapshot.swing.cancel")}
      onConfirm={onConfirm}
    />
  );
}

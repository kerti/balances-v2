import { useTranslation } from "react-i18next";
import type { FormMode } from "./mode";

// The one line under a field that is fixed once the thing exists (currency,
// liability subtype, snapshot month). Edit renders it where the disabled field
// sits; Create renders nothing.
export function LockedFieldHint({ mode, id }: { mode: FormMode; id: string }) {
  const { t } = useTranslation("common");
  if (mode !== "edit") return null;
  return (
    <p className="text-xs text-muted-foreground" data-testid={`locked-hint-${id}`}>
      {t("fields.lockedHint")}
    </p>
  );
}

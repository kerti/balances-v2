import { useTranslation } from "react-i18next";
import type { FormMode } from "./mode";

// The one line under a field that is fixed once the thing exists (currency,
// liability subtype, snapshot month). Edit renders it where the disabled field
// sits, under the control so a two-column row's labels stay level (#948);
// Create renders nothing. A locked currency also names the way out (create a
// new one), said here once rather than again in the dialog description.
export function LockedFieldHint({
  mode,
  id,
  kind,
}: {
  mode: FormMode;
  id: string;
  kind?: "currency";
}) {
  const { t } = useTranslation("common");
  if (mode !== "edit") return null;
  return (
    <p className="text-xs text-muted-foreground" data-testid={`locked-hint-${id}`}>
      {t(kind === "currency" ? "fields.lockedCurrencyHint" : "fields.lockedHint")}
    </p>
  );
}

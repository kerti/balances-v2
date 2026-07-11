import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import type { RiskProfile } from "@/api/types";

// Risk Profile select: required, no default. The empty placeholder "— select
// —" forces the user to make a deliberate choice (per the M6 grilling — the
// whole point of the field is intentionality). Reused by all 5 investment
// Create + Edit dialogs; the prefix scopes the htmlFor/id pair so multiple
// instances can coexist on a page without colliding.

type Props = {
  /** Disambiguating prefix for the input id, e.g. "stock_create". */
  idPrefix: string;
  /** Empty string means "not yet selected" — the parent should refuse submit. */
  value: RiskProfile | "";
  onChange: (next: RiskProfile) => void;
};

export function RiskProfileSelect({ idPrefix, value, onChange }: Props) {
  const { t } = useTranslation("investments");
  const id = `${idPrefix}_risk_profile`;
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{t("riskProfile.selectLabel")}</Label>
      <select
        id={id}
        required
        className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        value={value}
        onChange={(e) => onChange(e.target.value as RiskProfile)}
      >
        <option value="" disabled>
          {t("riskProfile.selectPlaceholder")}
        </option>
        <option value="low">{t("riskProfile.selectLow")}</option>
        <option value="medium">{t("riskProfile.selectMedium")}</option>
        <option value="high">{t("riskProfile.selectHigh")}</option>
      </select>
    </div>
  );
}

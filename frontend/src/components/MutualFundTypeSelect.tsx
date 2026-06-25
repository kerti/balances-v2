import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import type { MutualFundType } from "@/api/types";

// Fund Type select: required, no default (issue #20). The empty placeholder
// "— select —" forces a deliberate choice, matching RiskProfileSelect. The
// option order tracks the DB CHECK in migration 00023: the four universal
// asset classes first, then the structural wrappers, then `other`. The prefix
// scopes the htmlFor/id pair so the Create and Edit dialogs can coexist.

// Kept in DB-CHECK order so the dropdown reads asset-class → wrapper → other.
const ORDER: MutualFundType[] = [
  "money_market",
  "fixed_income",
  "equity",
  "mixed",
  "index",
  "etf",
  "target_date",
  "commodity",
  "other",
];

type Props = {
  /** Disambiguating prefix for the input id, e.g. "mf_create". */
  idPrefix: string;
  /** Empty string means "not yet selected" — the parent should refuse submit. */
  value: MutualFundType | "";
  onChange: (next: MutualFundType) => void;
};

export function MutualFundTypeSelect({ idPrefix, value, onChange }: Props) {
  const { t } = useTranslation("investments");
  const id = `${idPrefix}_fund_type`;
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{t("mutualFund.fundType.selectLabel")}</Label>
      <select
        id={id}
        required
        className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        value={value}
        onChange={(e) => onChange(e.target.value as MutualFundType)}
      >
        <option value="" disabled>
          {t("mutualFund.fundType.selectPlaceholder")}
        </option>
        {ORDER.map((ft) => (
          <option key={ft} value={ft}>
            {t(`mutualFund.fundType.option.${ft}`)}
          </option>
        ))}
      </select>
    </div>
  );
}

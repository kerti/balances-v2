import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import type { RiskProfile } from "@/api/types";

export type RiskProfileFilterValue = "all" | RiskProfile;

const OPTIONS: { value: RiskProfileFilterValue; labelKey: string }[] = [
  { value: "all", labelKey: "riskProfile.filterAll" },
  { value: "low", labelKey: "riskProfile.filterLow" },
  { value: "medium", labelKey: "riskProfile.filterMedium" },
  { value: "high", labelKey: "riskProfile.filterHigh" },
];

type Props = {
  value: RiskProfileFilterValue;
  onChange: (next: RiskProfileFilterValue) => void;
};

// Chip-bar filter mounted on each of the 5 per-subtype list screens
// (stocks/MFs/golds/bonds/TDs). Mirrors the regularity filter pattern on the
// Income screen — Button variant toggles between default (selected) and
// outline (idle).
export function RiskProfileFilter({ value, onChange }: Props) {
  const { t } = useTranslation("investments");
  return (
    <div className="flex gap-2" role="group" aria-label={t("riskProfile.filterAriaLabel")}>
      {OPTIONS.map((opt) => (
        <Button
          key={opt.value}
          size="sm"
          variant={value === opt.value ? "default" : "outline"}
          onClick={() => onChange(opt.value)}
          data-testid={`risk-filter-${opt.value}`}
        >
          {t(opt.labelKey)}
        </Button>
      ))}
    </div>
  );
}

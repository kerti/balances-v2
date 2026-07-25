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
    // On phones the chip bar fills the width and the options split it evenly
    // (`max-md:[&>*]:flex-1`), each clearing the 44px tap floor (#542); from
    // 768px up it collapses back to natural, content-width chips.
    <div
      className="flex gap-2 max-md:w-full max-md:[&>*]:flex-1"
      role="group"
      aria-label={t("riskProfile.filterAriaLabel")}
    >
      {OPTIONS.map((opt) => (
        <Button
          key={opt.value}
          size="sm"
          variant={value === opt.value ? "default" : "outline"}
          onClick={() => onChange(opt.value)}
          className="min-h-11 md:min-h-0"
          data-testid={`risk-filter-${opt.value}`}
        >
          {t(opt.labelKey)}
        </Button>
      ))}
    </div>
  );
}

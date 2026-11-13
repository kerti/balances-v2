import { useTranslation } from "react-i18next";
import { SegmentedControl } from "@/components/ui/segmented-control";
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

// Segmented filter mounted on each of the 5 per-subtype list screens
// (stocks/MFs/golds/bonds/TDs). On phones it fills the width and the options
// split it evenly, each clearing the 44px tap floor (#542); from 768px up it
// collapses back to natural, content-width segments.
export function RiskProfileFilter({ value, onChange }: Props) {
  const { t } = useTranslation("investments");
  return (
    <SegmentedControl
      ariaLabel={t("riskProfile.filterAriaLabel")}
      value={value}
      onChange={onChange}
      options={OPTIONS.map((opt) => ({
        value: opt.value,
        label: t(opt.labelKey),
        testId: `risk-filter-${opt.value}`,
      }))}
    />
  );
}

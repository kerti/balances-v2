import { useTranslation } from "react-i18next";
import { SegmentedControl } from "@/components/ui/segmented-control";
import type { RangePreset } from "@/lib/chartRange";

type Props = {
  chips: RangePreset[];
  pressed: RangePreset | null;
  onPick: (p: RangePreset) => void;
};

// The 1Y / 3Y / 5Y / All chip row above a time-series chart. Renders nothing
// for a short history (no chips to offer).
export function RangeChips({ chips, pressed, onPick }: Props) {
  const { t } = useTranslation("common");
  if (chips.length === 0) return null;
  return (
    <SegmentedControl
      ariaLabel={t("chartRange.ariaLabel")}
      testId="chart-range-chips"
      value={pressed}
      onChange={onPick}
      options={chips.map((p) => ({
        value: p,
        testId: `chart-range-${p}`,
        title: t(`chartRange.label${p}`),
        label: (
          <>
            {t(`chartRange.${p}`)}
            {/* Visible text must lead the accessible name (axe label-content-name-mismatch). */}
            <span className="sr-only">, {t(`chartRange.label${p}`)}</span>
          </>
        ),
      }))}
    />
  );
}

import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
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
    <div
      className="flex gap-2 max-md:[&>*]:flex-1"
      role="group"
      aria-label={t("chartRange.ariaLabel")}
      data-testid="chart-range-chips"
    >
      {chips.map((p) => {
        const on = pressed === p;
        return (
          <Button
            key={p}
            size="sm"
            variant={on ? "default" : "outline"}
            aria-pressed={on}
            title={t(`chartRange.label${p}`)}
            onClick={() => onPick(p)}
            data-testid={`chart-range-${p}`}
          >
            {t(`chartRange.${p}`)}
            {/* Visible text must lead the accessible name (axe label-content-name-mismatch). */}
            <span className="sr-only">, {t(`chartRange.label${p}`)}</span>
          </Button>
        );
      })}
    </div>
  );
}

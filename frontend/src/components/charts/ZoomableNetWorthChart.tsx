import { useState, type ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { SnapshotChart } from "@/components/charts/SnapshotChart";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  RANGE_PRESETS,
  loadRangePreset,
  presetWindow,
  saveRangePreset,
  shiftToInclude,
  type IndexWindow,
  type RangePreset,
} from "@/lib/chartRange";
import { monthRange } from "@/lib/months";

type Props = Omit<ComponentProps<typeof SnapshotChart>, "interaction"> & {
  // Selected month as "YYYY-MM"; the chart marks it and clicking a point
  // reports the clicked month back through onSelectMonth.
  selectedMonth: string;
  onSelectMonth: (yearMonth: string) => void;
};

// The dashboard's net-worth chart with range zoom (#852): 1Y / 3Y / 5Y / All
// chips everywhere, plus a drag-to-zoom Brush strip from 768px up. The chip
// choice is remembered per browser; a dragged window is not (it has no chip).
export function ZoomableNetWorthChart({ selectedMonth, onSelectMonth, ...chartProps }: Props) {
  const { t } = useTranslation("dashboard");
  const isMobile = useIsMobile();
  const [preset, setPreset] = useState<RangePreset>(loadRangePreset);
  // A window set by the Brush (or by shifting to the selected month). While
  // null, `preset` is in force and its chip reads as pressed.
  const [custom, setCustom] = useState<IndexWindow | null>(null);

  const yms = chartProps.snapshots.map((s) => s.year_month.slice(0, 7)).sort();
  const months = yms.length > 0 ? monthRange(yms[0], yms[yms.length - 1]) : [];
  const count = months.length;
  const selectedIdx = months.indexOf(selectedMonth.slice(0, 7));

  const base = custom ?? presetWindow(preset, count);

  // When the picked month changes (picker or click), shift the window to keep
  // it in view. Done at the change, not on every render, so dragging the Brush
  // away from the selected month is not fought.
  const [prevSelected, setPrevSelected] = useState(selectedMonth);
  if (prevSelected !== selectedMonth) {
    setPrevSelected(selectedMonth);
    const next = shiftToInclude(base, selectedIdx, count);
    if (next !== base) setCustom(next);
  }

  const win: IndexWindow = {
    start: Math.min(base.start, Math.max(0, count - 1)),
    end: Math.min(base.end, Math.max(0, count - 1)),
  };

  function pickPreset(p: RangePreset) {
    setPreset(p);
    setCustom(null);
    saveRangePreset(p);
  }

  return (
    <div className="space-y-3">
      <div
        className="flex gap-2 max-md:[&>*]:flex-1"
        role="group"
        aria-label={t("chart.range.ariaLabel")}
        data-testid="chart-range-chips"
      >
        {RANGE_PRESETS.map((p) => {
          const pressed = custom === null && preset === p;
          return (
            <Button
              key={p}
              size="sm"
              variant={pressed ? "default" : "outline"}
              aria-pressed={pressed}
              title={t(`chart.range.label${p}`)}
              onClick={() => pickPreset(p)}
              data-testid={`chart-range-${p}`}
            >
              {t(`chart.range.${p}`)}
              {/* Visible text must lead the accessible name (axe label-content-name-mismatch). */}
              <span className="sr-only">, {t(`chart.range.label${p}`)}</span>
            </Button>
          );
        })}
      </div>
      <SnapshotChart
        {...chartProps}
        interaction={{
          window: win,
          showBrush: !isMobile,
          onWindowChange: setCustom,
          selectedMonth: selectedMonth.slice(0, 7),
          onSelectMonth,
        }}
      />
    </div>
  );
}

import { useState } from "react";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  effectivePreset,
  loadRangePreset,
  presetWindow,
  saveRangePreset,
  shiftToInclude,
  visiblePresets,
  type IndexWindow,
  type RangePreset,
} from "@/lib/chartRange";

// The Brush needs at least this many months to be worth dragging.
const MIN_BRUSH_MONTHS = 3;

type Selection = { key: string; index: number };

// Range-zoom state shared by every monthly time-series chart (#852, #901):
// the chip choice (one per browser, shared by all charts), a Brush-dragged
// window (not remembered), and which chips a history of `count` months offers.
// `selection` is the dashboard-only selected month: when it changes the window
// slides to keep it in view.
export function useRangeZoom(count: number, selection?: Selection) {
  const isMobile = useIsMobile();
  const [stored, setStored] = useState<RangePreset>(loadRangePreset);
  // A window set by the Brush (or by shifting to the selected month). While
  // null, the preset is in force and its chip reads as pressed.
  const [custom, setCustom] = useState<IndexWindow | null>(null);

  const chips = visiblePresets(count);
  const preset = effectivePreset(stored, count);
  const base = custom ?? presetWindow(preset, count);

  // Shift at the change, not on every render, so dragging the Brush away from
  // the selected month is not fought.
  const [prevKey, setPrevKey] = useState(selection?.key);
  if (selection && prevKey !== selection.key) {
    setPrevKey(selection.key);
    const next = shiftToInclude(base, selection.index, count);
    if (next !== base) setCustom(next);
  }

  const last = Math.max(0, count - 1);
  const win: IndexWindow = {
    start: Math.min(base.start, last),
    end: Math.min(base.end, last),
  };

  function pickPreset(p: RangePreset) {
    setStored(p);
    setCustom(null);
    saveRangePreset(p);
  }

  return {
    chips,
    // The pressed chip, or null once a dragged window is in force.
    pressed: custom === null ? preset : null,
    pickPreset,
    window: win,
    onWindowChange: setCustom,
    showBrush: !isMobile && count >= MIN_BRUSH_MONTHS,
  };
}

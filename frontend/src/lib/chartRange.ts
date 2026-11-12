// Range zoom for the monthly time-series charts (#852, #901): the 1Y / 3Y / 5Y
// / All chips, the Brush window, and the one remembered choice. Pure, so the window
// arithmetic is testable without rendering a chart.

export const RANGE_PRESETS = ["1Y", "3Y", "5Y", "All"] as const;
export type RangePreset = (typeof RANGE_PRESETS)[number];
export const DEFAULT_RANGE_PRESET: RangePreset = "5Y";

// Inclusive indices into the chart's month list.
export type IndexWindow = { start: number; end: number };

const PRESET_MONTHS: Record<Exclude<RangePreset, "All">, number> = {
  "1Y": 12,
  "3Y": 36,
  "5Y": 60,
};

// Which chips to offer for a history of `count` months: a chip only when the
// history is longer than it covers (otherwise it would show the same thing as
// All), All always, and no chip row at all at 12 months or fewer.
export function visiblePresets(count: number): RangePreset[] {
  if (count <= PRESET_MONTHS["1Y"]) return [];
  return RANGE_PRESETS.filter((p) => p === "All" || count > PRESET_MONTHS[p]);
}

// The remembered preset in force for this history: a stored chip that is
// hidden for a short history reads as All (without touching the stored choice).
export function effectivePreset(preset: RangePreset, count: number): RangePreset {
  return visiblePresets(count).includes(preset) ? preset : "All";
}

// The rows inside the window (clamped to the data).
export function sliceWindow<T>(rows: T[], win: IndexWindow | undefined): T[] {
  return win ? rows.slice(win.start, win.end + 1) : rows;
}

// The most recent N months (all of them for "All", or when history is shorter).
export function presetWindow(preset: RangePreset, count: number): IndexWindow {
  const end = Math.max(0, count - 1);
  if (preset === "All") return { start: 0, end };
  return { start: Math.max(0, count - PRESET_MONTHS[preset]), end };
}

// Slide the window (keeping its width) just far enough to contain `index`.
export function shiftToInclude(win: IndexWindow, index: number, count: number): IndexWindow {
  if (index < 0 || index >= count || (index >= win.start && index <= win.end)) return win;
  const width = win.end - win.start;
  if (index < win.start) return { start: index, end: Math.min(count - 1, index + width) };
  return { start: Math.max(0, index - width), end: index };
}

export const RANGE_STORAGE_KEY = "balances.dashboard.chartRange";

export function isRangePreset(v: unknown): v is RangePreset {
  return (RANGE_PRESETS as readonly unknown[]).includes(v);
}

// localStorage can throw or come back empty (private windows, blocked site
// data), so every access is guarded and the default wins on any failure.
export function loadRangePreset(): RangePreset {
  try {
    const v = localStorage.getItem(RANGE_STORAGE_KEY);
    return isRangePreset(v) ? v : DEFAULT_RANGE_PRESET;
  } catch {
    return DEFAULT_RANGE_PRESET;
  }
}

export function saveRangePreset(preset: RangePreset): void {
  try {
    localStorage.setItem(RANGE_STORAGE_KEY, preset);
  } catch {
    // Remembering the choice is a convenience; ignore storage failures.
  }
}

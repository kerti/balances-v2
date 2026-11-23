// The fixed swatch palette a Tag's colour is chosen from (ADR-0028). Stored
// verbatim as the Tag's `color` (a Tailwind 500-level hex), so a tag keeps a
// stable hue across the pie, the table, and the chips wherever it appears.
// Ten visually distinct hues — enough that a household rarely needs to repeat
// one, few enough to stay glanceable. No red: red means debt and negatives
// only (ADR-0054), so indigo took its slot (#950). New swatches only ever append.
export const TAG_SWATCHES = [
  "#3b82f6", // blue
  "#06b6d4", // cyan
  "#10b981", // emerald
  "#84cc16", // lime
  "#eab308", // yellow
  "#f97316", // orange
  "#6366f1", // indigo
  "#ec4899", // pink
  "#a855f7", // violet
  "#64748b", // slate
] as const;

export const DEFAULT_TAG_COLOR = TAG_SWATCHES[0];

// The muted hue used for the Untagged bucket in the report — deliberately not
// in TAG_SWATCHES so it never collides with a real tag.
export const UNTAGGED_COLOR = "#94a3b8"; // slate-400

// Tags saved before red left the palette still store this hex. It is never
// written to the DB again; every place that draws a tag colour maps it here.
const RETIRED_RED = "#ef4444";
const RED_REPLACEMENT = "#6366f1"; // indigo

export function displayTagColor(color: string): string {
  return color.toLowerCase() === RETIRED_RED ? RED_REPLACEMENT : color;
}

// Purity comes from the backend as a decimal fraction string (e.g. "0.9999",
// "0.7500"). Display as karat when it cleanly divides by 24/x, otherwise
// fall through to a percentage. Pure (0.9999+) is conventionally called
// 24K even though strictly 24K means 1.0000.

export function formatGoldPurity(purity: string): string {
  const n = Number(purity);
  if (Number.isNaN(n) || n <= 0 || n > 1) return purity;
  if (n >= 0.999) return "24K (.999+)";
  const karat = n * 24;
  const rounded = Math.round(karat);
  if (Math.abs(karat - rounded) < 0.01) {
    return `${rounded}K`;
  }
  return `${(n * 100).toFixed(2)}%`;
}

// Karat presets for the purity picker. 24K maps to 0.9999 (Antam-bar fine gold)
// rather than 1.0000 — the conventional bullion figure. The non-24 values are
// k/24 rounded to 4 dp, matching the clean-karat detection in formatGoldPurity.
// A purity that doesn't sit on a preset (e.g. a 0.999 bar, distinct from a
// 0.9999 Antam bar) falls through to the Custom free-text path, so the picker
// never silently coarsens a stored value.
export const GOLD_PURITY_PRESETS: ReadonlyArray<{
  karat: number;
  value: string;
}> = [
  { karat: 24, value: "0.9999" },
  { karat: 22, value: "0.9167" },
  { karat: 20, value: "0.8333" },
  { karat: 18, value: "0.7500" },
  { karat: 14, value: "0.5833" },
  { karat: 10, value: "0.4167" },
];

// Returns the matching preset karat for a stored purity, or null when the value
// sits off-grid and should be edited as Custom. Numeric compare with a tight
// epsilon so "0.75" matches the "0.7500" preset but "0.999" stays off 24K's
// "0.9999".
export function goldPurityPresetKarat(purity: string): number | null {
  const n = Number(purity);
  if (Number.isNaN(n)) return null;
  const hit = GOLD_PURITY_PRESETS.find(
    (p) => Math.abs(Number(p.value) - n) < 1e-4,
  );
  return hit ? hit.karat : null;
}

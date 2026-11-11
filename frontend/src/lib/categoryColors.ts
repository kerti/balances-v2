// System category colours for the group overview charts (ADR-0054 amendment
// 2026-11-09). Red and green mean direction (--positive / --negative), so no
// category uses either hue. The values live as theme tokens in index.css
// (--cat-*, light and dark); this module is the one place components look
// them up. Tag swatches are the household's own choice and stay in tagColors.ts.

export const ASSET_CATEGORY_COLORS = {
  bankAccount: "var(--cat-bank-account)",
  property: "var(--cat-property)",
  vehicle: "var(--cat-vehicle)",
} as const;

export const LIABILITY_CATEGORY_COLORS = {
  personal: "var(--cat-personal)",
  institutional: "var(--cat-institutional)",
} as const;

export const INVESTMENT_CATEGORY_COLORS = {
  stock: "var(--cat-stock)",
  mutualFund: "var(--cat-mutual-fund)",
  bond: "var(--cat-bond)",
  timeDeposit: "var(--cat-time-deposit)",
  gold: "var(--cat-gold)",
} as const;

// Risk is ordinal, not good or bad to own: one brass hue, more ink for more risk
// (lighter in dark mode) — never a traffic light.
export const RISK_PROFILE_COLORS = {
  low: "var(--risk-low)",
  medium: "var(--risk-medium)",
  high: "var(--risk-high)",
} as const;

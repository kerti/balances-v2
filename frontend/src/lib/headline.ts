// Shared visual treatment for a screen's summary "headline" surface — the
// primary-tinted card at the top of a screen (net worth, income total, the
// per-group hub totals, the per-list totals). Centralised so the look stays
// consistent across every screen; the Income headline (#508) is the reference
// this was lifted from. Compose with each site's own layout classes via `cn`:
//   cn("rounded-lg border p-4", headlineSurface)
export const headlineSurface = "border-primary/30 bg-primary/5 shadow-sm dark:bg-primary/10";

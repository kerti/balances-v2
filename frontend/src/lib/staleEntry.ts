import { positionEntry, routes } from "@/lib/routes";
import type { MonthlyReport } from "@/api/types";

// The bulk-entry screens in the order the stale warning links to them: the
// first one with a stale position wins, whatever the report's own ordering.
// One definition of "stale" (the report's stale_positions, #50) serves both the
// dashboard link (#792) and the next-group toast after a bulk save (#805).
export const STALE_ENTRY_ORDER = [
  routes.assetsEnter,
  routes.liabilitiesEnter,
  routes.receivablesEnter,
  routes.investmentsEnterPrices,
  routes.investmentsEnterAccrued,
];

export function staleEntryRoute(stale: MonthlyReport["stale_positions"]): string | null {
  const present = new Set(stale.map((p) => positionEntry(p.group, p.subtype)));
  return STALE_ENTRY_ORDER.find((r) => present.has(r)) ?? null;
}

// The nav.json key naming each entry screen's group. Both investment screens
// are "Investments" to the user.
const GROUP_NAV_KEY: Record<string, string> = {
  [routes.assetsEnter]: "assets",
  [routes.liabilitiesEnter]: "liabilities",
  [routes.receivablesEnter]: "receivables",
  [routes.investmentsEnterPrices]: "investments",
  [routes.investmentsEnterAccrued]: "investments",
};

export type NextStaleEntry = { route: string; navKey: string; count: number };

// nextStaleEntry is the first entry screen, other than the one just saved, that
// still has stale positions in the report, with how many. Null when nothing
// else is stale.
export function nextStaleEntry(
  stale: MonthlyReport["stale_positions"],
  currentRoute: string,
): NextStaleEntry | null {
  const counts = new Map<string, number>();
  for (const p of stale) {
    const r = positionEntry(p.group, p.subtype);
    if (r) counts.set(r, (counts.get(r) ?? 0) + 1);
  }
  const route = STALE_ENTRY_ORDER.find((r) => r !== currentRoute && counts.has(r));
  if (!route) return null;
  return { route, navKey: GROUP_NAV_KEY[route], count: counts.get(route)! };
}

// Helpers for the Tags page's per-tag item lists and Add-items picker (#853).
import type { TaggableItem } from "@/api/types";
import { routes } from "@/lib/routes";

// The detail page for an item, from its (group, subtype). Null for an unknown
// pair so a caller renders plain text instead of a dead link.
export function itemDetailPath(
  item: Pick<TaggableItem, "group" | "subtype" | "id">,
): string | null {
  const { group, subtype, id } = item;
  switch (group) {
    case "asset":
      if (subtype === "bank_account") return routes.bankAccount(id);
      if (subtype === "property") return routes.property(id);
      if (subtype === "vehicle") return routes.vehicle(id);
      return null;
    case "liability":
      if (subtype === "personal" || subtype === "institutional") {
        return routes.liability(subtype, id);
      }
      return null;
    case "receivable":
      return routes.receivable(id);
    case "investment":
      if (subtype === "stock") return routes.stock(id);
      if (subtype === "mutual_fund") return routes.mutualFund(id);
      if (subtype === "bond") return routes.bond(id);
      if (subtype === "gold") return routes.goldItem(id);
      if (subtype === "time_deposit") return routes.timeDeposit(id);
      return null;
  }
}

// Stable key for an item across the four tables (ids are only unique per table).
export const itemKey = (item: Pick<TaggableItem, "group" | "id">) => `${item.group}:${item.id}`;

// The order the picker groups types in: assets, investments, receivables,
// liabilities (holdings first, what is owed last), subtype order within each.
const TYPE_ORDER = [
  "bank_account",
  "property",
  "vehicle",
  "stock",
  "mutual_fund",
  "bond",
  "gold",
  "time_deposit",
  "receivable",
  "personal",
  "institutional",
];

export type ItemTypeGroup = { subtype: string; items: TaggableItem[] };

// Groups items by type (subtype), types in TYPE_ORDER, items by name. Unknown
// subtypes sort last under their own heading.
export function groupItemsByType(items: TaggableItem[]): ItemTypeGroup[] {
  const bySubtype = new Map<string, TaggableItem[]>();
  for (const it of items) {
    const list = bySubtype.get(it.subtype) ?? [];
    list.push(it);
    bySubtype.set(it.subtype, list);
  }
  const rank = (s: string) => {
    const i = TYPE_ORDER.indexOf(s);
    return i === -1 ? TYPE_ORDER.length : i;
  };
  return [...bySubtype.entries()]
    .sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
    .map(([subtype, list]) => ({
      subtype,
      items: [...list].sort((a, b) => a.name.localeCompare(b.name)),
    }));
}

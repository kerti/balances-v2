import { Landmark, Home, Car, User, Building2, type LucideIcon } from "lucide-react";
import { routes } from "@/lib/routes";
import type { EntryDataConfig } from "@/hooks/useBulkEntry";

// Per-group config for the bulk monthly-entry screen (ADR-0046). Extends the
// data-layer EntryDataConfig (endpoints + id field) with the presentation the
// EntryScreen needs: where its Back/Cancel return to, its testid prefix, and
// how to group + label rows by subtype. A flat group (receivables) sets
// `labelNs: null` with an empty subtype order — the screen then renders one
// ungrouped list.
export type SubtypeMeta = { icon: LucideIcon; labelKey: string };

export type EntryGroupConfig = EntryDataConfig & {
  // Where the screen's Back/Cancel and a committed Save return to.
  backRoute: string;
  // Prefix for every data-testid the screen emits, e.g. "asset" →
  // "asset-entry-row-…". Kept per-group so the Asset tracer's testids (#421)
  // are unchanged.
  testidPrefix: string;
  // i18n namespace whose `home.categoryLabel.*` keys label each subtype group;
  // null for a flat group with no subtypes.
  labelNs: string | null;
  // The subtypes to present, in order; [] for a flat group.
  subtypeOrder: string[];
  subtypeMeta: Record<string, SubtypeMeta>;
};

// Assets — three subtypes, grouped. Preserves the #421 tracer's endpoints,
// id field, testid prefix, and invalidation set unchanged.
export const assetEntryConfig: EntryGroupConfig = {
  group: "assets",
  apiBase: "/api/assets/snapshots",
  idField: "asset_id",
  invalidateKeys: [["assets"], ["bank-accounts"]],
  backRoute: routes.assets,
  testidPrefix: "asset",
  labelNs: "assets",
  subtypeOrder: ["bank_account", "property", "vehicle"],
  subtypeMeta: {
    bank_account: { icon: Landmark, labelKey: "bankAccount" },
    property: { icon: Home, labelKey: "property" },
    vehicle: { icon: Car, labelKey: "vehicle" },
  },
};

// Liabilities — two subtypes, grouped. Invalidates the liability lists + the
// Home time-series chart so carried-forward totals refresh after a save.
export const liabilityEntryConfig: EntryGroupConfig = {
  group: "liabilities",
  apiBase: "/api/liabilities/snapshots",
  idField: "liability_id",
  invalidateKeys: [["liabilities"], ["liability-time-series"]],
  backRoute: routes.liabilities,
  testidPrefix: "liability",
  labelNs: "liabilities",
  subtypeOrder: ["personal", "institutional"],
  subtypeMeta: {
    personal: { icon: User, labelKey: "personal" },
    institutional: { icon: Building2, labelKey: "institutional" },
  },
};

// Receivables — a flat group with no subtype; one ungrouped list.
export const receivableEntryConfig: EntryGroupConfig = {
  group: "receivables",
  apiBase: "/api/receivables/snapshots",
  idField: "receivable_id",
  invalidateKeys: [["receivables"], ["receivable-time-series"]],
  backRoute: routes.receivables,
  testidPrefix: "receivable",
  labelNs: null,
  subtypeOrder: [],
  subtypeMeta: {},
};

import {
  Landmark,
  Home,
  Car,
  User,
  Building2,
  LineChart,
  PieChart,
  Coins,
  ScrollText,
  PiggyBank,
  type LucideIcon,
} from "lucide-react";
import { routes } from "@/lib/routes";
import type { EntryDataConfig, EntryRow } from "@/hooks/useBulkEntry";
import {
  amountOnlyShape,
  qtyPriceShape,
  accruedShape,
  timeDepositShape,
  type EntryShape,
} from "@/components/entry/shapes";

// Per-group config for the bulk monthly-entry screen (ADR-0046). Extends the
// data-layer EntryDataConfig (endpoints + id field) with the presentation the
// EntryScreen needs: where its Back/Cancel return to, its testid prefix, and
// how to group + label rows by subtype. A flat group (receivables) sets
// `labelNs: null` with an empty subtype order — the screen then renders one
// ungrouped list.
export type SubtypeMeta = { icon: LucideIcon; labelKey: string };

export type EntryGroupConfig = EntryDataConfig & {
  // The row's editable shape (ADR-0022): amount-only for Asset/Liability/
  // Receivable, qty×price for the Stock/MutualFund/Gold group (#423). Drives
  // which inputs a row renders and how Save maps them to wire columns.
  shape: EntryShape;
  // Picks a row's own shape when the group mixes shapes (the accrued group: a
  // bond takes two figures, a time deposit one, #844). Absent ⇒ `shape` for every
  // row.
  shapeFor?: (row: EntryRow) => EntryShape;
  // i18n key prefix (common namespace) for the screen's title / description /
  // empty / rowError copy. Defaults to "bulkEntry" (account-centric wording);
  // the investment group overrides to "bulkEntry.investments" (holdings wording).
  copyPrefix?: string;
  // This screen's own route: the next-group toast after a save (#805) skips it.
  enterRoute: string;
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
  enterRoute: routes.assetsEnter,
  shape: amountOnlyShape,
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
  enterRoute: routes.liabilitiesEnter,
  shape: amountOnlyShape,
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
  enterRoute: routes.receivablesEnter,
  shape: amountOnlyShape,
  apiBase: "/api/receivables/snapshots",
  idField: "receivable_id",
  invalidateKeys: [["receivables"], ["receivable-time-series"]],
  backRoute: routes.receivables,
  testidPrefix: "receivable",
  labelNs: null,
  subtypeOrder: [],
  subtypeMeta: {},
};

// Investments (qty×price shape, #423) — the three subtypes whose snapshots take
// quantity + price per unit: Stock, MutualFund, Gold. Grouped by subtype, with
// holdings-oriented copy (copyPrefix). Invalidates the three lists + the
// investment Home time-series so carried-forward totals refresh after a save.
// Bond/TimeDeposit (accrued shape) are a separate entry view (#424), not here.
export const investmentEntryConfig: EntryGroupConfig = {
  group: "investments",
  enterRoute: routes.investmentsEnterPrices,
  shape: qtyPriceShape,
  copyPrefix: "bulkEntry.investments",
  apiBase: "/api/investments/snapshots",
  idField: "investment_id",
  invalidateKeys: [["stocks"], ["mutual-funds"], ["golds"], ["investment-time-series"]],
  backRoute: routes.investments,
  testidPrefix: "investment",
  labelNs: "investments",
  subtypeOrder: ["stock", "mutual_fund", "gold"],
  subtypeMeta: {
    stock: { icon: LineChart, labelKey: "stock" },
    mutual_fund: { icon: PieChart, labelKey: "mutualFund" },
    gold: { icon: Coins, labelKey: "gold" },
  },
};

// Investments (accrued shape, #424) — the two subtypes whose snapshots take the
// accrued_interest branch: Bond, TimeDeposit. A bond row carries the total value +
// accrued interest (the per-position accrued dialog's two figures); a time deposit
// row carries one Value and its interest is derived (#844). Distinct
// `group` id (own cache namespace) and `apiBase` (own /accrued/entry + /bulk
// endpoints) so it never collides with the qty×price view. Invalidates the two
// lists + the investment Home time-series so carried-forward totals refresh.
// Stock/MutualFund/Gold (qty×price shape) live in investmentEntryConfig above.
export const investmentAccruedEntryConfig: EntryGroupConfig = {
  group: "investments-accrued",
  enterRoute: routes.investmentsEnterAccrued,
  shape: accruedShape,
  shapeFor: (row) =>
    row.subtype === "time_deposit" ? timeDepositShape(row.principal) : accruedShape,
  copyPrefix: "bulkEntry.investmentsAccrued",
  apiBase: "/api/investments/snapshots/accrued",
  idField: "investment_id",
  invalidateKeys: [["bonds"], ["time-deposits"], ["investment-time-series"]],
  backRoute: routes.investments,
  testidPrefix: "investment-accrued",
  labelNs: "investments",
  subtypeOrder: ["bond", "time_deposit"],
  subtypeMeta: {
    bond: { icon: ScrollText, labelKey: "bond" },
    time_deposit: { icon: PiggyBank, labelKey: "timeDeposit" },
  },
};

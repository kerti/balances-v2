// Where a snapshot lives, derived from the parent id its API row carries. The
// Move action is mounted inside every Edit snapshot dialog, whatever the shape,
// so it works out its group's routes and query keys from the row itself instead
// of taking a fourth mutation prop through the three row hooks.

export type SnapshotGroup = {
  /** `/api/<group>/{position}/snapshots`, the list route and the move route's parent. */
  basePath: string;
  /** The React Query key of this position's snapshot list (shared with the detail page). */
  listKey: readonly unknown[];
  /** Position-list queries that inline the latest snapshot per row. */
  positionKeys: readonly (readonly unknown[])[];
};

const ASSET_LISTS = [["bank-accounts"], ["properties"], ["vehicles"]] as const;
const INVESTMENT_LISTS = [
  ["stocks"],
  ["mutual-funds"],
  ["golds"],
  ["bonds"],
  ["time-deposits"],
] as const;

function idOf(snapshot: object, field: string): string | null {
  const value = (snapshot as Record<string, unknown>)[field];
  return typeof value === "string" && value !== "" ? value : null;
}

export function snapshotGroupOf(snapshot: object): SnapshotGroup | null {
  const asset = idOf(snapshot, "asset_id");
  if (asset) {
    return {
      basePath: `/api/assets/${asset}/snapshots`,
      listKey: ["snapshots", asset],
      positionKeys: ASSET_LISTS,
    };
  }
  const liability = idOf(snapshot, "liability_id");
  if (liability) {
    return {
      basePath: `/api/liabilities/${liability}/snapshots`,
      listKey: ["liability-snapshots", liability],
      positionKeys: [["liabilities"]],
    };
  }
  const receivable = idOf(snapshot, "receivable_id");
  if (receivable) {
    return {
      basePath: `/api/receivables/${receivable}/snapshots`,
      listKey: ["receivable-snapshots", receivable],
      positionKeys: [["receivables"]],
    };
  }
  const investment = idOf(snapshot, "investment_id");
  if (investment) {
    return {
      basePath: `/api/investments/${investment}/snapshots`,
      listKey: ["investment-snapshots", investment],
      positionKeys: INVESTMENT_LISTS,
    };
  }
  return null;
}

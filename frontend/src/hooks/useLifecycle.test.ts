import { describe, expect, it } from "vitest";
import { lifecycleInvalidationKeys } from "@/hooks/useLifecycle";

// useUpdateLifecycle is glue over react-query; the meaningful decision — which
// caches a lifecycle change must refresh — lives in this pure helper (the repo
// has no jsdom/RTL runner, ADR-0021). The regression these pin is issue #56: a
// terminal flip writes a 0-value close snapshot (INV-LIFECYCLE-03) that must
// appear in the snapshot list without a manual reload. ADR-0052 generalised that
// write from investments to all four groups, so all four must invalidate — and
// the keys are not uniformly named, which is the part a typo would silently
// break.

const ID = "11111111-1111-1111-1111-111111111111";

// covers: INV-LIFECYCLE-03
describe("lifecycleInvalidationKeys", () => {
  it.each([
    ["investments", "stocks", "investment-snapshots"],
    ["assets", "assets", "snapshots"],
    ["liabilities", "liabilities", "liability-snapshots"],
    ["receivables", "receivables", "receivable-snapshots"],
  ] as const)(
    "refreshes the %s snapshot list on a terminal flip (issue #56)",
    (group, listKey, snapshotKey) => {
      const keys = lifecycleInvalidationKeys(group, ID, listKey);
      expect(keys.slice(0, 3)).toEqual([[listKey], [listKey, ID], [snapshotKey, ID]]);
    },
  );

  // covers: INV-LIFECYCLE-08
  it("also refreshes the ledger for investments, which alone can settle on the flip", () => {
    // Terminating an Investment can write its settling Sell/Maturity in the same
    // request (ADR-0052 §6), so the transaction table the detail page renders is
    // stale the moment the flip lands. The other three groups have no ledger.
    expect(lifecycleInvalidationKeys("investments", ID, "stocks")).toContainEqual([
      "investment-transactions",
      ID,
    ]);
    for (const group of ["assets", "liabilities", "receivables"] as const) {
      expect(lifecycleInvalidationKeys(group, ID, group)).toHaveLength(3);
    }
  });
});

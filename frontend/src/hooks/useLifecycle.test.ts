import { describe, expect, it } from "vitest";
import { lifecycleInvalidationKeys } from "@/hooks/useLifecycle";

// useUpdateLifecycle is glue over react-query; the meaningful decision — which
// caches a lifecycle change must refresh — lives in this pure helper (the repo
// has no jsdom/RTL runner, ADR-0021). The regression these pin is issue #56: an
// investment terminal flip writes a 0-value close snapshot (INV-LIFECYCLE-03)
// that must appear in the snapshot list without a manual reload.

const ID = "11111111-1111-1111-1111-111111111111";

describe("lifecycleInvalidationKeys", () => {
  it("refreshes the investment snapshot list on a terminal flip (issue #56)", () => {
    const keys = lifecycleInvalidationKeys("investments", ID, "stocks");
    expect(keys).toEqual([
      ["stocks"],
      ["stocks", ID],
      ["investment-snapshots", ID],
    ]);
  });

  it.each(["assets", "liabilities", "receivables"] as const)(
    "leaves the snapshot list alone for %s (no close snapshot written)",
    (group) => {
      const keys = lifecycleInvalidationKeys(group, ID, group);
      expect(keys).toEqual([[group], [group, ID]]);
      expect(keys).not.toContainEqual(["investment-snapshots", ID]);
    },
  );
});

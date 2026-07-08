import { describe, it, expect } from "vitest";
import { computeDonutSlices } from "@/lib/pdf/charts/pieChartMath";

describe("computeDonutSlices", () => {
  it("returns no slices for an empty input", () => {
    expect(computeDonutSlices([], { cx: 50, cy: 50, rOuter: 40, rInner: 20 })).toEqual([]);
  });

  it("returns no slices when every value is zero", () => {
    const out = computeDonutSlices(
      [
        { key: "a", value: 0 },
        { key: "b", value: 0 },
      ],
      { cx: 50, cy: 50, rOuter: 40, rInner: 20 },
    );
    expect(out).toEqual([]);
  });

  it("drops zero-value entries but keeps the rest", () => {
    const out = computeDonutSlices(
      [
        { key: "a", value: 100 },
        { key: "b", value: 0 },
        { key: "c", value: 50 },
      ],
      { cx: 50, cy: 50, rOuter: 40, rInner: 20 },
    );
    expect(out.map((s) => s.key)).toEqual(["a", "c"]);
  });

  it("produces one non-empty path per positive-value slice", () => {
    const out = computeDonutSlices(
      [
        { key: "a", value: 30 },
        { key: "b", value: 70 },
      ],
      { cx: 50, cy: 50, rOuter: 40, rInner: 20 },
    );
    expect(out).toHaveLength(2);
    for (const slice of out) {
      expect(slice.path).toMatch(/^M .+ A .+ L .+ A .+ Z$/);
    }
  });

  it("renders a single 100% slice without degenerating to an empty path", () => {
    const out = computeDonutSlices([{ key: "only", value: 42 }], {
      cx: 50,
      cy: 50,
      rOuter: 40,
      rInner: 20,
    });
    expect(out).toHaveLength(1);
    expect(out[0].path.length).toBeGreaterThan(10);
  });
});

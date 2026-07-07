import { describe, it, expect } from "vitest";
import { computeLineChartGeometry } from "@/lib/pdf/charts/lineChartMath";

describe("computeLineChartGeometry", () => {
  it("returns an empty geometry for an empty series", () => {
    const geo = computeLineChartGeometry([], { width: 200, height: 100 });
    expect(geo).toEqual({ path: "", points: [], minY: 0, maxY: 0 });
  });

  it("places a single point centered vertically at x=0", () => {
    const geo = computeLineChartGeometry([{ year_month: "2026-06", amount: "100000000" }], {
      width: 200,
      height: 100,
    });
    expect(geo.minY).toBe(100000000);
    expect(geo.maxY).toBe(100000000);
    expect(geo.points).toEqual([{ x: 0, y: 50 }]);
    expect(geo.path).toBe("M 0 50");
  });

  it("spans the full width and inverts y so a higher amount sits nearer the top", () => {
    const geo = computeLineChartGeometry(
      [
        { year_month: "2026-05", amount: "0" },
        { year_month: "2026-06", amount: "100" },
      ],
      { width: 200, height: 100 },
    );
    expect(geo.minY).toBe(0);
    expect(geo.maxY).toBe(100);
    expect(geo.points).toEqual([
      { x: 0, y: 100 }, // amount 0 -> bottom of the chart (y = height)
      { x: 200, y: 0 }, // amount 100 (max) -> top of the chart (y = 0)
    ]);
    expect(geo.path).toBe("M 0 100 L 200 0");
  });

  it("fills gap months by carrying the last known amount forward", () => {
    const geo = computeLineChartGeometry(
      [
        { year_month: "2026-04", amount: "100" },
        // May is skipped — no snapshot that month.
        { year_month: "2026-06", amount: "200" },
      ],
      { width: 200, height: 100 },
    );
    // 3 months in range now (Apr/May/Jun), evenly spaced across the width,
    // May carries forward April's amount rather than collapsing the gap.
    expect(geo.points).toEqual([
      { x: 0, y: 100 }, // Apr: 100 (min) -> bottom
      { x: 100, y: 100 }, // May: carried forward -> same y as Apr
      { x: 200, y: 0 }, // Jun: 200 (max) -> top
    ]);
  });
});

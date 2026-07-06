import { describe, it, expect } from "vitest";
import { aggregateListPositions, type Position } from "@/lib/listAggregates";

const pos = (overrides: Partial<Position> & { id: string }): Position => ({
  currency: "IDR",
  status: "active",
  terminated_at: null,
  latestValue: 0,
  cost: 0,
  snapshots: [],
  costSeries: [],
  ...overrides,
});

describe("aggregateListPositions — byCurrency totals", () => {
  it("returns empty totals on empty input", () => {
    const r = aggregateListPositions([]);
    expect(r.byCurrency).toEqual([]);
    expect(r.timeSeriesByCurrency.size).toBe(0);
    expect(r.count).toBe(0);
  });

  it("sums value + cost per currency for active positions", () => {
    const r = aggregateListPositions([
      pos({ id: "a", currency: "IDR", latestValue: 1000, cost: 800 }),
      pos({ id: "b", currency: "IDR", latestValue: 500, cost: 600 }),
      pos({ id: "c", currency: "USD", latestValue: 100, cost: 90 }),
    ]);
    expect(r.byCurrency).toEqual([
      { currency: "IDR", value: 1500, cost: 1400, pl: 100 },
      { currency: "USD", value: 100, cost: 90, pl: 10 },
    ]);
    expect(r.count).toBe(3);
  });

  it("excludes terminated positions from both totals + count", () => {
    const r = aggregateListPositions([
      pos({ id: "a", currency: "IDR", latestValue: 1000, cost: 800 }),
      pos({
        id: "b",
        currency: "IDR",
        status: "sold",
        latestValue: 9999,
        cost: 9999,
      }),
    ]);
    expect(r.byCurrency).toEqual([{ currency: "IDR", value: 1000, cost: 800, pl: 200 }]);
    expect(r.count).toBe(1);
  });

  it("counts cost basis even when a position has no value yet", () => {
    // A freshly-placed bond (its Buy recorded) with no snapshot yet still
    // contributes its cost to the headline (you've already committed
    // the money), but does not contribute to the value column.
    const r = aggregateListPositions([
      pos({ id: "a", currency: "IDR", latestValue: null, cost: 500 }),
    ]);
    expect(r.byCurrency).toEqual([{ currency: "IDR", value: 0, cost: 500, pl: -500 }]);
    expect(r.count).toBe(0);
  });

  it("orders byCurrency by value desc, then currency code", () => {
    const r = aggregateListPositions([
      pos({ id: "a", currency: "USD", latestValue: 100, cost: 0 }),
      pos({ id: "b", currency: "IDR", latestValue: 100, cost: 0 }),
    ]);
    expect(r.byCurrency.map((c) => c.currency)).toEqual(["IDR", "USD"]);
  });
});

describe("aggregateListPositions — time series", () => {
  it("emits a single-currency series with month-aligned sums", () => {
    const r = aggregateListPositions([
      pos({
        id: "a",
        latestValue: 200,
        cost: 100,
        snapshots: [
          { year_month: "2026-01", amount: "120" },
          { year_month: "2026-02", amount: "200" },
        ],
        costSeries: [
          { year_month: "2026-01", cost: 100 },
          { year_month: "2026-02", cost: 100 },
        ],
      }),
      pos({
        id: "b",
        latestValue: 60,
        cost: 50,
        snapshots: [{ year_month: "2026-02", amount: "60" }],
        costSeries: [{ year_month: "2026-02", cost: 50 }],
      }),
    ]);
    const idr = r.timeSeriesByCurrency.get("IDR");
    expect(idr).toEqual([
      // Jan: only A has a snapshot → 120 / 100. B not yet on the chart.
      { year_month: "2026-01", value: 120, cost: 100 },
      // Feb: A=200 + B=60 = 260; cost = 100 + 50 = 150.
      { year_month: "2026-02", value: 260, cost: 150 },
    ]);
  });

  it("records a cost month that precedes the first snapshot (value 0)", () => {
    // A buy can land in a month before the position's first value snapshot —
    // the cost point then has no snapshot to merge with, so the month enters
    // the series at value 0 with the carried cost.
    const r = aggregateListPositions([
      pos({
        id: "a",
        latestValue: 200,
        cost: 50,
        snapshots: [{ year_month: "2026-02", amount: "200" }],
        costSeries: [
          { year_month: "2026-01", cost: 50 },
          { year_month: "2026-02", cost: 50 },
        ],
      }),
    ]);
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2026-01", value: 0, cost: 50 },
      { year_month: "2026-02", value: 200, cost: 50 },
    ]);
  });

  it("carry-forwards the previous value when no current-month snapshot", () => {
    const r = aggregateListPositions([
      pos({
        id: "a",
        latestValue: 100,
        cost: 100,
        snapshots: [{ year_month: "2026-01", amount: "100" }],
        costSeries: [{ year_month: "2026-01", cost: 100 }],
      }),
      pos({
        id: "b",
        latestValue: 50,
        cost: 30,
        snapshots: [
          { year_month: "2026-01", amount: "40" },
          { year_month: "2026-02", amount: "50" },
        ],
        costSeries: [
          { year_month: "2026-01", cost: 30 },
          { year_month: "2026-02", cost: 30 },
        ],
      }),
    ]);
    const idr = r.timeSeriesByCurrency.get("IDR")!;
    // Feb: A carries its Jan 100; B has Feb 50. Total = 150.
    expect(idr[1]).toEqual({ year_month: "2026-02", value: 150, cost: 130 });
  });

  it("fills skipped months with carry-forward, keeping the timeline whole (#24)", () => {
    // A single position snapshotted in Jan and again in Apr — the two
    // gap months (Feb, Mar) must still appear, carrying Jan's value
    // forward, so the categorical X axis stays proportional.
    const r = aggregateListPositions([
      pos({
        id: "a",
        latestValue: 200,
        cost: 150,
        snapshots: [
          { year_month: "2026-01", amount: "100" },
          { year_month: "2026-04", amount: "200" },
        ],
        costSeries: [
          { year_month: "2026-01", cost: 100 },
          { year_month: "2026-04", cost: 150 },
        ],
      }),
    ]);
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2026-01", value: 100, cost: 100 },
      { year_month: "2026-02", value: 100, cost: 100 },
      { year_month: "2026-03", value: 100, cost: 100 },
      { year_month: "2026-04", value: 200, cost: 150 },
    ]);
  });

  it("separates currencies into independent series maps", () => {
    const r = aggregateListPositions([
      pos({
        id: "a",
        currency: "IDR",
        latestValue: 100,
        cost: 100,
        snapshots: [{ year_month: "2026-01", amount: "100" }],
        costSeries: [{ year_month: "2026-01", cost: 100 }],
      }),
      pos({
        id: "b",
        currency: "USD",
        latestValue: 50,
        cost: 40,
        snapshots: [{ year_month: "2026-01", amount: "50" }],
        costSeries: [{ year_month: "2026-01", cost: 40 }],
      }),
    ]);
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2026-01", value: 100, cost: 100 },
    ]);
    expect(r.timeSeriesByCurrency.get("USD")).toEqual([
      { year_month: "2026-01", value: 50, cost: 40 },
    ]);
  });

  it("accepts API year_month formatted as YYYY-MM-DDTHH", () => {
    const r = aggregateListPositions([
      pos({
        id: "a",
        latestValue: 100,
        cost: 100,
        snapshots: [{ year_month: "2026-01-01T00:00:00Z", amount: "100" }],
        costSeries: [{ year_month: "2026-01-01T00:00:00Z", cost: 100 }],
      }),
    ]);
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2026-01", value: 100, cost: 100 },
    ]);
  });

  it("omits currencies whose only positions have no snapshots from the series map", () => {
    const r = aggregateListPositions([
      pos({ id: "a", currency: "IDR", latestValue: null, cost: 500 }),
    ]);
    // byCurrency still records the cost; series map skips empty.
    expect(r.byCurrency).toHaveLength(1);
    expect(r.timeSeriesByCurrency.has("IDR")).toBe(false);
  });

  it("includes closed positions historically, capped before terminated_at", () => {
    // Issue #21: a sold/matured position must still show in the time series
    // for the months it was held, then drop out. Option A: it contributes
    // through the month BEFORE its termination month, then leaves — the
    // termination-month snapshot is its synthetic 0-close (#25/#27), excluded
    // so a same-month successor (rollover) doesn't double-count.
    const r = aggregateListPositions([
      pos({
        id: "active",
        latestValue: 100,
        cost: 100,
        snapshots: [
          { year_month: "2026-01", amount: "100" },
          { year_month: "2026-02", amount: "100" },
          { year_month: "2026-03", amount: "100" },
        ],
        costSeries: [
          { year_month: "2026-01", cost: 100 },
          { year_month: "2026-02", cost: 100 },
          { year_month: "2026-03", cost: 100 },
        ],
      }),
      pos({
        id: "sold",
        status: "sold",
        // Position terminated mid-February, carrying a 0-value close
        // snapshot at its termination month (#25).
        terminated_at: "2026-02-15T00:00:00Z",
        latestValue: 0,
        cost: 0,
        snapshots: [
          { year_month: "2026-01", amount: "200" },
          { year_month: "2026-02", amount: "0" },
        ],
        costSeries: [
          { year_month: "2026-01", cost: 150 },
          { year_month: "2026-02", cost: 0 },
        ],
      }),
    ]);
    // Closed position is excluded from the headline (active-only).
    expect(r.byCurrency).toEqual([{ currency: "IDR", value: 100, cost: 100, pl: 0 }]);
    expect(r.count).toBe(1);
    // Appears in Jan (held). From its termination month (Feb) on, it's gone —
    // the active position alone carries Feb + Mar. No phantom carry of the
    // sold value into Feb.
    const idr = r.timeSeriesByCurrency.get("IDR")!;
    expect(idr).toEqual([
      { year_month: "2026-01", value: 300, cost: 250 },
      { year_month: "2026-02", value: 100, cost: 100 },
      { year_month: "2026-03", value: 100, cost: 100 },
    ]);
  });

  it("does not crater the line at a matured position’s 0-close month (#24)", () => {
    // A lone matured position must not pull the summed line down to 0 at its
    // termination month. Option A: its termination month (the 0-close) is
    // excluded from the range entirely, so the series ends at the last live
    // month — no dip to 0, no phantom point.
    const r = aggregateListPositions([
      pos({
        id: "matured-td",
        status: "matured",
        terminated_at: "2026-03-10T00:00:00Z",
        latestValue: 0,
        cost: 0,
        snapshots: [
          { year_month: "2026-01", amount: "1000" },
          { year_month: "2026-02", amount: "1000" },
          { year_month: "2026-03", amount: "0" }, // #25 close snapshot
        ],
        costSeries: [
          { year_month: "2026-01", cost: 1000 },
          { year_month: "2026-02", cost: 1000 },
          { year_month: "2026-03", cost: 0 },
        ],
      }),
    ]);
    // With the 0-close dropped, the only real snapshots are Jan + Feb, so
    // the series ends at the last real value — no dip to 0, no phantom March
    // point. Matches the detail chart, which ends at the last real value and
    // marks it Matured.
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2026-01", value: 1000, cost: 1000 },
      { year_month: "2026-02", value: 1000, cost: 1000 },
    ]);
  });

  it("does not double-count a same-month rollover seam (Option A)", () => {
    // The real TD rollover case: R0 matures end of May (#25 0-close at May),
    // R1 is placed the same day at the rolled-over value. The seam month must
    // show R1 only — carrying R0 through May too would spike it to ~2× value.
    const r = aggregateListPositions([
      pos({
        id: "R0",
        status: "matured",
        terminated_at: "2023-05-31T00:00:00Z",
        latestValue: 0,
        cost: 0,
        snapshots: [
          { year_month: "2023-03", amount: "24000000" },
          { year_month: "2023-04", amount: "24000000" },
          { year_month: "2023-05", amount: "0" }, // #25 close
        ],
        costSeries: [
          { year_month: "2023-03", cost: 24000000 },
          { year_month: "2023-04", cost: 24000000 },
          { year_month: "2023-05", cost: 0 },
        ],
      }),
      pos({
        id: "R1",
        latestValue: 24576116,
        cost: 24576116,
        snapshots: [
          { year_month: "2023-05", amount: "24576116" },
          { year_month: "2023-06", amount: "24576116" },
        ],
        costSeries: [
          { year_month: "2023-05", cost: 24576116 },
          { year_month: "2023-06", cost: 24576116 },
        ],
      }),
    ]);
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2023-03", value: 24000000, cost: 24000000 },
      { year_month: "2023-04", value: 24000000, cost: 24000000 },
      // Seam: R0 gone (its termination month), R1 only — no spike.
      { year_month: "2023-05", value: 24576116, cost: 24576116 },
      { year_month: "2023-06", value: 24576116, cost: 24576116 },
    ]);
  });

  it("omits closed-position contribution from its termination month onward", () => {
    const r = aggregateListPositions([
      pos({
        id: "sold",
        status: "sold",
        // Held through Jan; terminated in Feb (with the #25 Feb 0-close).
        terminated_at: "2026-02-28T00:00:00Z",
        snapshots: [
          { year_month: "2026-01", amount: "500" },
          { year_month: "2026-02", amount: "0" },
        ],
        costSeries: [
          { year_month: "2026-01", cost: 400 },
          { year_month: "2026-02", cost: 0 },
        ],
      }),
      pos({
        id: "active",
        latestValue: 10,
        cost: 10,
        snapshots: [{ year_month: "2026-02", amount: "10" }],
        costSeries: [{ year_month: "2026-02", cost: 10 }],
      }),
    ]);
    const idr = r.timeSeriesByCurrency.get("IDR")!;
    // Jan: sold contributes 500/400; active not yet.
    expect(idr[0]).toEqual({ year_month: "2026-01", value: 500, cost: 400 });
    // Feb: sold dropped from its termination month (0-close excluded, no
    // carry of the Jan value); only active remains.
    expect(idr[1]).toEqual({ year_month: "2026-02", value: 10, cost: 10 });
  });
});

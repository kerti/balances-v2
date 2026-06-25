import { describe, it, expect } from "vitest";
import {
  aggregateHomePositions,
  type HomePosition,
} from "@/lib/homeAggregates";

const pos = (
  overrides: Partial<HomePosition> & { id: string },
): HomePosition => ({
  currency: "IDR",
  status: "active",
  terminated_at: null,
  latestValue: 0,
  cost: 0,
  snapshots: [],
  costSeries: [],
  category: "stock",
  riskProfile: "medium",
  ...overrides,
});

describe("aggregateHomePositions", () => {
  it("returns empty outputs on empty input", () => {
    const r = aggregateHomePositions([]);
    expect(r.byCurrency).toEqual([]);
    expect(r.timeSeriesByCurrency.size).toBe(0);
    expect(r.categorySeriesByCurrency.size).toBe(0);
    expect(r.categoryPieByCurrency.size).toBe(0);
    expect(r.riskPieByCurrency.size).toBe(0);
    expect(r.count).toBe(0);
  });

  it("forwards headline + value/cost time series to listAggregates", () => {
    const r = aggregateHomePositions([
      pos({
        id: "a",
        category: "stock",
        latestValue: 200,
        cost: 100,
        snapshots: [{ year_month: "2026-01", amount: "200" }],
        costSeries: [{ year_month: "2026-01", cost: 100 }],
      }),
      pos({
        id: "b",
        category: "bond",
        latestValue: 50,
        cost: 40,
        snapshots: [{ year_month: "2026-01", amount: "50" }],
        costSeries: [{ year_month: "2026-01", cost: 40 }],
      }),
    ]);
    expect(r.byCurrency).toEqual([
      { currency: "IDR", value: 250, cost: 140, pl: 110 },
    ]);
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2026-01", value: 250, cost: 140 },
    ]);
    expect(r.count).toBe(2);
  });

  it("breaks the monthly stacked series down by category", () => {
    const r = aggregateHomePositions([
      pos({
        id: "a",
        category: "stock",
        latestValue: 200,
        snapshots: [
          { year_month: "2026-01", amount: "100" },
          { year_month: "2026-02", amount: "200" },
        ],
      }),
      pos({
        id: "b",
        category: "bond",
        latestValue: 50,
        snapshots: [{ year_month: "2026-02", amount: "50" }],
      }),
    ]);
    const idr = r.categorySeriesByCurrency.get("IDR")!;
    expect(idr).toEqual([
      {
        year_month: "2026-01",
        byCategory: {
          stock: 100,
          mutualFund: 0,
          bond: 0,
          timeDeposit: 0,
          gold: 0,
        },
      },
      {
        year_month: "2026-02",
        byCategory: {
          stock: 200,
          mutualFund: 0,
          bond: 50,
          timeDeposit: 0,
          gold: 0,
        },
      },
    ]);
  });

  it("carries category snapshots forward into months without new data", () => {
    const r = aggregateHomePositions([
      pos({
        id: "a",
        category: "gold",
        latestValue: 1000,
        snapshots: [{ year_month: "2026-01", amount: "1000" }],
      }),
      pos({
        id: "b",
        category: "stock",
        latestValue: 500,
        snapshots: [{ year_month: "2026-02", amount: "500" }],
      }),
    ]);
    const idr = r.categorySeriesByCurrency.get("IDR")!;
    // Feb: gold carries the Jan 1000; stock contributes 500.
    expect(idr[1]).toEqual({
      year_month: "2026-02",
      byCategory: {
        stock: 500,
        mutualFund: 0,
        bond: 0,
        timeDeposit: 0,
        gold: 1000,
      },
    });
  });

  it("emits a category pie with all 5 keys, summed per category", () => {
    const r = aggregateHomePositions([
      pos({ id: "a", category: "stock", latestValue: 600 }),
      pos({ id: "b", category: "stock", latestValue: 400 }),
      pos({ id: "c", category: "bond", latestValue: 200 }),
    ]);
    expect(r.categoryPieByCurrency.get("IDR")).toEqual([
      { category: "stock", value: 1000 },
      { category: "mutualFund", value: 0 },
      { category: "bond", value: 200 },
      { category: "timeDeposit", value: 0 },
      { category: "gold", value: 0 },
    ]);
  });

  it("emits a risk pie with all 3 profiles, summed per profile", () => {
    const r = aggregateHomePositions([
      pos({ id: "a", riskProfile: "low", latestValue: 100 }),
      pos({ id: "b", riskProfile: "medium", latestValue: 250 }),
      pos({ id: "c", riskProfile: "medium", latestValue: 50 }),
      pos({ id: "d", riskProfile: "high", latestValue: 400 }),
    ]);
    expect(r.riskPieByCurrency.get("IDR")).toEqual([
      { profile: "low", value: 100 },
      { profile: "medium", value: 300 },
      { profile: "high", value: 400 },
    ]);
  });

  it("skips positions with a null latestValue in both pies", () => {
    // A position that has never been snapshotted carries latestValue === null;
    // it must not contribute to (nor zero out) the current category/risk mix.
    const r = aggregateHomePositions([
      pos({ id: "a", category: "stock", riskProfile: "low", latestValue: 500 }),
      pos({
        id: "b",
        category: "stock",
        riskProfile: "low",
        latestValue: null,
      }),
    ]);
    expect(r.categoryPieByCurrency.get("IDR")).toContainEqual({
      category: "stock",
      value: 500,
    });
    expect(r.riskPieByCurrency.get("IDR")).toContainEqual({
      profile: "low",
      value: 500,
    });
  });

  it("excludes terminated positions from headline + pies but keeps them in time + category series", () => {
    // Issue #21: closed positions show historically up to the month BEFORE
    // their terminated_at month, then drop out (Option A — the termination
    // month is when they convert to cash/successor). Pies + headline reflect
    // current state only, so closed positions are excluded there.
    const r = aggregateHomePositions([
      pos({
        id: "a",
        category: "stock",
        latestValue: 100,
        snapshots: [
          { year_month: "2026-01", amount: "100" },
          { year_month: "2026-02", amount: "100" },
        ],
      }),
      pos({
        id: "b",
        status: "sold",
        // Held through Jan, terminated in Feb (its Feb 0-close is excluded).
        terminated_at: "2026-02-15T00:00:00Z",
        category: "bond",
        latestValue: 9999,
        snapshots: [
          { year_month: "2026-01", amount: "300" },
          { year_month: "2026-02", amount: "0" },
        ],
      }),
    ]);
    expect(r.byCurrency).toEqual([
      { currency: "IDR", value: 100, cost: 0, pl: 100 },
    ]);
    expect(r.categoryPieByCurrency.get("IDR")).toEqual([
      { category: "stock", value: 100 },
      { category: "mutualFund", value: 0 },
      { category: "bond", value: 0 },
      { category: "timeDeposit", value: 0 },
      { category: "gold", value: 0 },
    ]);
    // Category stack: Jan includes both; Feb only the active stock.
    const idr = r.categorySeriesByCurrency.get("IDR")!;
    expect(idr).toEqual([
      {
        year_month: "2026-01",
        byCategory: {
          stock: 100,
          mutualFund: 0,
          bond: 300,
          timeDeposit: 0,
          gold: 0,
        },
      },
      {
        year_month: "2026-02",
        byCategory: {
          stock: 100,
          mutualFund: 0,
          bond: 0,
          timeDeposit: 0,
          gold: 0,
        },
      },
    ]);
  });

  it("keeps currencies separate", () => {
    const r = aggregateHomePositions([
      pos({
        id: "a",
        currency: "IDR",
        category: "stock",
        latestValue: 100,
        snapshots: [{ year_month: "2026-01", amount: "100" }],
      }),
      pos({
        id: "b",
        currency: "USD",
        category: "bond",
        latestValue: 50,
        snapshots: [{ year_month: "2026-01", amount: "50" }],
      }),
    ]);
    expect(r.byCurrency).toEqual([
      { currency: "IDR", value: 100, cost: 0, pl: 100 },
      { currency: "USD", value: 50, cost: 0, pl: 50 },
    ]);
    expect(
      r.categoryPieByCurrency.get("IDR")!.find((s) => s.category === "stock")
        ?.value,
    ).toBe(100);
    expect(
      r.categoryPieByCurrency.get("USD")!.find((s) => s.category === "bond")
        ?.value,
    ).toBe(50);
    expect(r.categorySeriesByCurrency.has("USD")).toBe(true);
  });

  it("accepts API year_month formatted as YYYY-MM-DDTHH", () => {
    const r = aggregateHomePositions([
      pos({
        id: "a",
        category: "stock",
        latestValue: 100,
        snapshots: [{ year_month: "2026-01-01T00:00:00Z", amount: "100" }],
      }),
    ]);
    const idr = r.categorySeriesByCurrency.get("IDR")!;
    expect(idr[0].year_month).toBe("2026-01");
    expect(idr[0].byCategory.stock).toBe(100);
  });
});

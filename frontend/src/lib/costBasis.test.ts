import { describe, it, expect } from "vitest";
import {
  computeCostBasis,
  costBasisSeries,
  flatCostSeries,
} from "@/lib/costBasis";
import type { InvestmentTransaction, TransactionType } from "@/api/types";

// Fixtures carry only the fields the helper reads (cast past the wire type).
const txn = (
  transaction_type: TransactionType,
  transaction_date: string,
  fields: {
    amount?: string | null;
    quantity?: string | null;
  } = {},
): InvestmentTransaction =>
  ({
    transaction_type,
    transaction_date,
    amount: fields.amount ?? null,
    quantity: fields.quantity ?? null,
  }) as InvestmentTransaction;

// covers: INV-COST-BASIS-01, INV-COST-BASIS-02
describe("computeCostBasis", () => {
  it("returns zero on an empty ledger", () => {
    expect(computeCostBasis([])).toEqual({ cost: 0, heldQty: 0 });
  });

  it("accumulates buys into cost + quantity", () => {
    const r = computeCostBasis([
      txn("buy", "2026-01-15", { amount: "1000", quantity: "10" }),
      txn("buy", "2026-02-10", { amount: "600", quantity: "5" }),
    ]);
    expect(r).toEqual({ cost: 1600, heldQty: 15 });
  });

  it("reduces both cost and qty proportionally on sell (avg-cost)", () => {
    // Buy 10 @ 100 = 1000; sell 4 @ any price.
    // Avg cost = 1000 / 10 = 100 per unit; remove 4 * 100 = 400 from cost.
    const r = computeCostBasis([
      txn("buy", "2026-01-15", { amount: "1000", quantity: "10" }),
      txn("sell", "2026-02-15", { amount: "500", quantity: "4" }),
    ]);
    expect(r.cost).toBeCloseTo(600, 6);
    expect(r.heldQty).toBeCloseTo(6, 6);
  });

  it("uses running avg-cost after a second buy at a different price", () => {
    // Buy 10 @ 100 = 1000; Buy 10 @ 200 = 2000 → avg = 150.
    // Sell 5 → remove 5 * 150 = 750 → cost = 2250, qty = 15.
    const r = computeCostBasis([
      txn("buy", "2026-01-01", { amount: "1000", quantity: "10" }),
      txn("buy", "2026-02-01", { amount: "2000", quantity: "10" }),
      txn("sell", "2026-03-01", { amount: "1100", quantity: "5" }),
    ]);
    expect(r.cost).toBeCloseTo(2250, 6);
    expect(r.heldQty).toBeCloseTo(15, 6);
  });

  it("capitalizes standalone fees into cost", () => {
    const r = computeCostBasis([
      txn("buy", "2026-01-01", { amount: "1000", quantity: "10" }),
      txn("fee", "2026-01-05", { amount: "25" }),
    ]);
    expect(r).toEqual({ cost: 1025, heldQty: 10 });
  });

  it("ignores coupon / dividend / distribution / maturity", () => {
    const r = computeCostBasis([
      txn("buy", "2026-01-01", { amount: "1000", quantity: "10" }),
      txn("coupon", "2026-02-01", { amount: "50" }),
      txn("dividend", "2026-03-01", { amount: "40" }),
      txn("distribution", "2026-04-01", { amount: "30" }),
      txn("maturity", "2026-05-01", { amount: "0" }),
    ]);
    expect(r).toEqual({ cost: 1000, heldQty: 10 });
  });

  it("caps oversell at currently-held qty (defensive)", () => {
    const r = computeCostBasis([
      txn("buy", "2026-01-01", { amount: "500", quantity: "5" }),
      txn("sell", "2026-02-01", { amount: "1000", quantity: "99" }),
    ]);
    expect(r.cost).toBeCloseTo(0, 6);
    expect(r.heldQty).toBeCloseTo(0, 6);
  });

  it("ignores sells when nothing is held", () => {
    const r = computeCostBasis([
      txn("sell", "2026-01-01", { amount: "100", quantity: "5" }),
    ]);
    expect(r).toEqual({ cost: 0, heldQty: 0 });
  });

  it("skips malformed buys (null amount or quantity)", () => {
    const r = computeCostBasis([
      txn("buy", "2026-01-01", { amount: null, quantity: "10" }),
      txn("buy", "2026-02-01", { amount: "1000", quantity: null }),
      txn("buy", "2026-03-01", { amount: "500", quantity: "5" }),
    ]);
    expect(r).toEqual({ cost: 500, heldQty: 5 });
  });

  it("replays in date order regardless of input order", () => {
    // If sell came before buy in input, naive iteration would no-op the
    // sell. Sort-by-date guarantees the buy lands first.
    const r = computeCostBasis([
      txn("sell", "2026-02-01", { amount: "500", quantity: "5" }),
      txn("buy", "2026-01-01", { amount: "1000", quantity: "10" }),
    ]);
    expect(r.cost).toBeCloseTo(500, 6);
    expect(r.heldQty).toBeCloseTo(5, 6);
  });

  it("ignores a sell with a missing/non-finite quantity", () => {
    const r = computeCostBasis([
      txn("buy", "2026-01-01", { amount: "1000", quantity: "10" }),
      txn("sell", "2026-02-01", { amount: "500", quantity: null }),
    ]);
    expect(r).toEqual({ cost: 1000, heldQty: 10 });
  });

  it("ignores a fee with a missing/non-finite amount", () => {
    const r = computeCostBasis([
      txn("buy", "2026-01-01", { amount: "1000", quantity: "10" }),
      txn("fee", "2026-02-01", { amount: null }),
    ]);
    expect(r).toEqual({ cost: 1000, heldQty: 10 });
  });
});

// covers: INV-COST-BASIS-03
describe("costBasisSeries", () => {
  it("returns one cost entry per snapshot in input order", () => {
    const snaps = [{ year_month: "2026-03" }, { year_month: "2026-01" }];
    const r = costBasisSeries(snaps, [
      txn("buy", "2026-01-15", { amount: "1000", quantity: "10" }),
    ]);
    // Order preserved; cost is constant 1000 from Jan onward.
    expect(r).toEqual([
      { year_month: "2026-03", cost: 1000 },
      { year_month: "2026-01", cost: 1000 },
    ]);
  });

  it("attributes a mid-month txn to that snapshot month", () => {
    const snaps = [
      { year_month: "2026-01" },
      { year_month: "2026-02" },
      { year_month: "2026-03" },
    ];
    const r = costBasisSeries(snaps, [
      txn("buy", "2026-02-15", { amount: "1000", quantity: "10" }),
    ]);
    expect(r).toEqual([
      { year_month: "2026-01", cost: 0 },
      { year_month: "2026-02", cost: 1000 },
      { year_month: "2026-03", cost: 1000 },
    ]);
  });

  it("accepts API year_month formatted as YYYY-MM-DDTHH", () => {
    const snaps = [{ year_month: "2026-02-01T00:00:00Z" }];
    const r = costBasisSeries(snaps, [
      txn("buy", "2026-02-15", { amount: "500", quantity: "5" }),
    ]);
    expect(r[0].cost).toBe(500);
  });

  it("reflects an interim sell at the correct snapshot", () => {
    const snaps = [
      { year_month: "2026-01" },
      { year_month: "2026-02" },
      { year_month: "2026-03" },
    ];
    const r = costBasisSeries(snaps, [
      txn("buy", "2026-01-15", { amount: "1000", quantity: "10" }),
      txn("sell", "2026-02-15", { amount: "600", quantity: "4" }),
    ]);
    expect(r[0].cost).toBeCloseTo(1000, 6);
    expect(r[1].cost).toBeCloseTo(600, 6);
    expect(r[2].cost).toBeCloseTo(600, 6);
  });
});

// covers: INV-COST-BASIS-03
describe("flatCostSeries", () => {
  it("emits the same cost at every snapshot", () => {
    const snaps = [{ year_month: "2026-01" }, { year_month: "2026-02" }];
    expect(flatCostSeries(snaps, 5000)).toEqual([
      { year_month: "2026-01", cost: 5000 },
      { year_month: "2026-02", cost: 5000 },
    ]);
  });

  it("returns [] for empty snapshots", () => {
    expect(flatCostSeries([], 5000)).toEqual([]);
  });
});

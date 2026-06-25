import { describe, it, expect } from "vitest";
import { activeCurrencyTotals } from "@/lib/totals";

const snap = (amount: string, currency = "IDR") => ({ amount, currency });

describe("activeCurrencyTotals", () => {
  it("sums active balances within a single currency", () => {
    const r = activeCurrencyTotals([
      { status: "active", snapshot: snap("1000") },
      { status: "active", snapshot: snap("2500") },
    ]);
    expect(r.totals).toEqual([{ currency: "IDR", amount: 3500 }]);
    expect(r.count).toBe(2);
  });

  it("excludes terminated positions", () => {
    const r = activeCurrencyTotals([
      { status: "active", snapshot: snap("1000") },
      { status: "closed", snapshot: snap("9999") },
      { status: "sold", snapshot: snap("5") },
    ]);
    expect(r.totals).toEqual([{ currency: "IDR", amount: 1000 }]);
    expect(r.count).toBe(1);
  });

  it("ignores positions with no snapshot", () => {
    const r = activeCurrencyTotals([
      { status: "active", snapshot: null },
      { status: "active", snapshot: snap("700") },
    ]);
    expect(r.totals).toEqual([{ currency: "IDR", amount: 700 }]);
    expect(r.count).toBe(1);
  });

  it("keeps currencies separate, largest first", () => {
    const r = activeCurrencyTotals([
      { status: "active", snapshot: snap("100", "USD") },
      { status: "active", snapshot: snap("50000000", "IDR") },
      { status: "active", snapshot: snap("50", "USD") },
    ]);
    expect(r.totals).toEqual([
      { currency: "IDR", amount: 50000000 },
      { currency: "USD", amount: 150 },
    ]);
    expect(r.count).toBe(3);
  });

  it("breaks equal-amount ties by currency code", () => {
    const r = activeCurrencyTotals([
      { status: "active", snapshot: snap("100", "USD") },
      { status: "active", snapshot: snap("100", "EUR") },
    ]);
    expect(r.totals.map((t) => t.currency)).toEqual(["EUR", "USD"]);
  });

  it("returns empty totals and zero count when nothing qualifies", () => {
    const r = activeCurrencyTotals([
      { status: "closed", snapshot: snap("100") },
      { status: "active", snapshot: null },
    ]);
    expect(r.totals).toEqual([]);
    expect(r.count).toBe(0);
  });
});

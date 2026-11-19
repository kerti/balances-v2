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

  it("counts an empty active position but adds nothing to the sums", () => {
    const r = activeCurrencyTotals([
      { status: "active", snapshot: null },
      { status: "active", snapshot: snap("700") },
    ]);
    expect(r.totals).toEqual([{ currency: "IDR", amount: 700 }]);
    expect(r.count).toBe(2);
  });

  // covers: INV-PRESENTATION-34
  it("keeps currencies separate, reporting currency first then A-Z", () => {
    const items = [
      { status: "active", snapshot: snap("50000000", "IDR") },
      { status: "active", snapshot: snap("100", "USD") },
      { status: "active", snapshot: snap("9", "EUR") },
      { status: "active", snapshot: snap("50", "USD") },
    ];
    const r = activeCurrencyTotals(items, "USD");
    expect(r.totals).toEqual([
      { currency: "USD", amount: 150 },
      { currency: "EUR", amount: 9 },
      { currency: "IDR", amount: 50000000 },
    ]);
    expect(r.count).toBe(4);
    expect(activeCurrencyTotals(items).totals.map((t) => t.currency)).toEqual([
      "EUR",
      "IDR",
      "USD",
    ]);
  });
});

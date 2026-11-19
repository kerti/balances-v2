import { describe, expect, it } from "vitest";
import {
  compareCurrencies,
  orderByCurrency,
  orderCurrencies,
  orderMapByCurrency,
} from "@/lib/currencyOrder";

// covers: INV-PRESENTATION-34
describe("currency order", () => {
  it("puts the reporting currency first, then the rest A-Z", () => {
    expect(orderCurrencies(["USD", "SGD", "IDR", "EUR"], "IDR")).toEqual([
      "IDR",
      "EUR",
      "SGD",
      "USD",
    ]);
    expect(orderCurrencies(["USD", "IDR", "EUR"], "USD")).toEqual(["USD", "EUR", "IDR"]);
  });

  it("is plain A-Z when the reporting currency is absent or empty", () => {
    expect(orderCurrencies(["USD", "EUR"], "IDR")).toEqual(["EUR", "USD"]);
    expect(orderCurrencies(["USD", "EUR"], "")).toEqual(["EUR", "USD"]);
  });

  it("handles empty and single-currency input", () => {
    expect(orderCurrencies([], "IDR")).toEqual([]);
    expect(orderCurrencies(["USD"], "IDR")).toEqual(["USD"]);
    expect(compareCurrencies("IDR")("IDR", "IDR")).toBe(0);
  });

  it("orders objects by .currency", () => {
    const rows = [{ currency: "USD" }, { currency: "IDR" }, { currency: "EUR" }];
    expect(orderByCurrency(rows, "IDR").map((r) => r.currency)).toEqual(["IDR", "EUR", "USD"]);
  });

  it("orders Map keys", () => {
    const m = new Map([
      ["USD", 1],
      ["IDR", 2],
    ]);
    expect([...orderMapByCurrency(m, "IDR").keys()]).toEqual(["IDR", "USD"]);
  });
});

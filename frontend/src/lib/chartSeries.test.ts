import { describe, expect, it } from "vitest";
import { seriesOrder, seriesRank, sortBySeriesOrder } from "./chartSeries";
import { COMPOSITION_ORDER } from "./netWorthComposition";

// covers: INV-PRESENTATION-15
describe("seriesOrder", () => {
  it("leads with the primary area, then cost when drawn, then the extras as given", () => {
    expect(seriesOrder(["assets", "investments", "liabilities"], false)).toEqual([
      "amount",
      "assets",
      "investments",
      "liabilities",
    ]);
    expect(seriesOrder([], true)).toEqual(["amount", "cost"]);
  });

  it("is the dashboard's legend order: Net worth, Assets, Investments, Liabilities", () => {
    expect(seriesOrder(COMPOSITION_ORDER, false)).toEqual([
      "amount",
      "assets",
      "investments",
      "liabilities",
    ]);
  });
});

describe("seriesRank", () => {
  it("ranks by position and sends an unknown key last", () => {
    const order = ["amount", "assets"];
    expect(seriesRank(order, "amount")).toBe(0);
    expect(seriesRank(order, "assets")).toBe(1);
    expect(seriesRank(order, "mystery")).toBe(2);
  });
});

// covers: INV-PRESENTATION-15
describe("sortBySeriesOrder", () => {
  it("puts a tooltip payload in the legend's order, whatever order it arrived in", () => {
    // The order Recharts handed the tooltip on the dashboard before the fix.
    const payload = [
      { dataKey: "amount" },
      { dataKey: "assets" },
      { dataKey: "liabilities" },
      { dataKey: "investments" },
    ];
    const order = seriesOrder(COMPOSITION_ORDER, false);
    expect(sortBySeriesOrder(payload, order, (p) => p.dataKey).map((p) => p.dataKey)).toEqual([
      "amount",
      "assets",
      "investments",
      "liabilities",
    ]);
  });

  it("does not mutate its input and keeps unknown keys last, in arrival order", () => {
    const payload = [{ k: "x" }, { k: "assets" }, { k: "y" }, { k: "amount" }];
    const copy = [...payload];
    const out = sortBySeriesOrder(payload, ["amount", "assets"], (p) => p.k);
    expect(out.map((p) => p.k)).toEqual(["amount", "assets", "x", "y"]);
    expect(payload).toEqual(copy);
  });
});

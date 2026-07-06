import { describe, it, expect } from "vitest";
import { aggregateGroupHome, type GroupPosition } from "@/lib/groupHomeAggregates";

const CATS = ["bankAccount", "property", "vehicle"];

const pos = (overrides: Partial<GroupPosition> & { id: string }): GroupPosition => ({
  currency: "IDR",
  status: "active",
  terminated_at: null,
  latestValue: 0,
  snapshots: [],
  category: "bankAccount",
  ...overrides,
});

describe("aggregateGroupHome", () => {
  it("returns empty outputs on empty input", () => {
    const r = aggregateGroupHome([], CATS);
    expect(r.byCurrency).toEqual([]);
    expect(r.timeSeriesByCurrency.size).toBe(0);
    expect(r.categorySeriesByCurrency.size).toBe(0);
    expect(r.categoryPieByCurrency.size).toBe(0);
    expect(r.count).toBe(0);
  });

  it("produces a value-only headline + total-over-time (cost always 0)", () => {
    const r = aggregateGroupHome(
      [
        pos({
          id: "a",
          category: "bankAccount",
          latestValue: 200,
          snapshots: [{ year_month: "2026-01", amount: "200" }],
        }),
        pos({
          id: "b",
          category: "property",
          latestValue: 50,
          snapshots: [{ year_month: "2026-01", amount: "50" }],
        }),
      ],
      CATS,
    );
    expect(r.byCurrency).toEqual([{ currency: "IDR", value: 250, cost: 0, pl: 250 }]);
    expect(r.timeSeriesByCurrency.get("IDR")).toEqual([
      { year_month: "2026-01", value: 250, cost: 0 },
    ]);
    expect(r.count).toBe(2);
  });

  it("breaks the monthly stacked series down by category with carry-forward", () => {
    const r = aggregateGroupHome(
      [
        pos({
          id: "a",
          category: "bankAccount",
          latestValue: 200,
          snapshots: [
            { year_month: "2026-01", amount: "100" },
            { year_month: "2026-02", amount: "200" },
          ],
        }),
        pos({
          id: "b",
          category: "property",
          latestValue: 50,
          // No Feb snapshot → Jan value carries forward into Feb.
          snapshots: [{ year_month: "2026-01", amount: "50" }],
        }),
      ],
      CATS,
    );
    const series = r.categorySeriesByCurrency.get("IDR")!;
    expect(series).toHaveLength(2);
    expect(series[0]).toEqual({
      year_month: "2026-01",
      byCategory: { bankAccount: 100, property: 50, vehicle: 0 },
    });
    expect(series[1]).toEqual({
      year_month: "2026-02",
      byCategory: { bankAccount: 200, property: 50, vehicle: 0 },
    });
  });

  it("emits every category key in the current pie, active-only", () => {
    const r = aggregateGroupHome(
      [
        pos({ id: "a", category: "bankAccount", latestValue: 300 }),
        pos({ id: "b", category: "vehicle", latestValue: 120 }),
        // Terminated → excluded from the active pie.
        pos({
          id: "c",
          category: "property",
          status: "sold",
          terminated_at: "2026-03-01",
          latestValue: 999,
        }),
      ],
      CATS,
    );
    expect(r.categoryPieByCurrency.get("IDR")).toEqual([
      { category: "bankAccount", value: 300 },
      { category: "property", value: 0 },
      { category: "vehicle", value: 120 },
    ]);
  });

  it("caps a terminated position at the month before terminated_at in the stack", () => {
    const r = aggregateGroupHome(
      [
        pos({
          id: "a",
          category: "bankAccount",
          status: "active",
          latestValue: 100,
          snapshots: [
            { year_month: "2026-01", amount: "100" },
            { year_month: "2026-02", amount: "100" },
            { year_month: "2026-03", amount: "100" },
          ],
        }),
        pos({
          id: "b",
          category: "property",
          status: "sold",
          terminated_at: "2026-03-01",
          latestValue: 0,
          snapshots: [
            { year_month: "2026-01", amount: "40" },
            { year_month: "2026-02", amount: "40" },
            { year_month: "2026-03", amount: "0" },
          ],
        }),
      ],
      CATS,
    );
    const series = r.categorySeriesByCurrency.get("IDR")!;
    // March: the sold property has dropped out, bank account remains.
    const march = series.find((p) => p.year_month === "2026-03")!;
    expect(march.byCategory).toEqual({
      bankAccount: 100,
      property: 0,
      vehicle: 0,
    });
  });
});

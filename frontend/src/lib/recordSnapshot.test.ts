import { describe, expect, it } from "vitest";
import {
  asOfInputValue,
  defaultRecordMonth,
  formatRecordMonth,
  holdingMonthLabel,
  lastRecordedBefore,
  latestHoldingPrice,
  snapshotForMonth,
} from "./recordSnapshot";

const at = (ym: string) => ({ year_month: `${ym}-01T00:00:00Z` });

describe("defaultRecordMonth", () => {
  it("is the current month with no snapshots", () => {
    expect(defaultRecordMonth([], "2026-11")).toBe("2026-11");
    expect(defaultRecordMonth(undefined, "2026-11")).toBe("2026-11");
  });

  it("is the month after the latest snapshot", () => {
    expect(defaultRecordMonth([at("2026-08"), at("2026-09")], "2026-11")).toBe("2026-10");
  });

  it("rolls over a year boundary", () => {
    expect(defaultRecordMonth([at("2025-12")], "2026-11")).toBe("2026-01");
  });

  it("is capped at the current month", () => {
    expect(defaultRecordMonth([at("2026-11")], "2026-11")).toBe("2026-11");
    expect(defaultRecordMonth([at("2026-12")], "2026-11")).toBe("2026-11");
  });

  it("does not assume the list is sorted", () => {
    expect(defaultRecordMonth([at("2026-03"), at("2026-09"), at("2026-05")], "2026-11")).toBe(
      "2026-10",
    );
  });
});

describe("snapshotForMonth / lastRecordedBefore", () => {
  const list = [
    { id: "c", ...at("2026-10") },
    { id: "b", ...at("2026-08") },
    { id: "a", ...at("2026-03") },
  ];

  it("finds the live snapshot for a bare or full month", () => {
    expect(snapshotForMonth(list, "2026-08")?.id).toBe("b");
    expect(snapshotForMonth(list, "2026-08-01T00:00:00Z")?.id).toBe("b");
    expect(snapshotForMonth(list, "2026-09")).toBeUndefined();
    expect(snapshotForMonth(undefined, "2026-09")).toBeUndefined();
  });

  it("finds the latest snapshot strictly before a month", () => {
    expect(lastRecordedBefore(list, "2026-11")?.id).toBe("c");
    expect(lastRecordedBefore(list, "2026-10")?.id).toBe("b");
    expect(lastRecordedBefore(list, "2026-09")?.id).toBe("b");
    expect(lastRecordedBefore(list, "2026-03")).toBeUndefined();
  });
});

describe("formatters", () => {
  it("formats a month as short month and year", () => {
    expect(formatRecordMonth("2026-11")).toBe("Nov 2026");
    expect(formatRecordMonth("2026-11-01T00:00:00Z")).toBe("Nov 2026");
  });

  it("slices the statement date for the date input", () => {
    expect(asOfInputValue("2026-11-05T00:00:00Z")).toBe("2026-11-05");
    expect(asOfInputValue(null)).toBe("");
  });
});

describe("holdingMonthLabel", () => {
  it("names the month Record snapshot will open on", () => {
    expect(holdingMonthLabel([at("2026-06")])).toBe("Jul");
  });
});

describe("latestHoldingPrice", () => {
  const snap = (ym: string, price: string | null) => ({ ...at(ym), price_per_unit: price });
  const trade = (type: string, date: string, price: string | null) => ({
    transaction_type: type,
    transaction_date: date,
    price_per_unit: price,
  });

  it("uses the latest snapshot's price, not a trade's execution price", () => {
    const price = latestHoldingPrice(
      [snap("2026-06", "100"), snap("2026-05", "90")],
      [trade("buy", "2026-07-02T00:00:00Z", "130")],
      "2026-07",
    );
    expect(price).toBe("100");
  });

  it("prefers the snapshot already in the opened month (replace mode)", () => {
    expect(
      latestHoldingPrice([snap("2026-07", "120"), snap("2026-06", "100")], [], "2026-07"),
    ).toBe("120");
  });

  it("ignores snapshots after the opened month", () => {
    expect(
      latestHoldingPrice([snap("2026-09", "200"), snap("2026-06", "100")], [], "2026-07"),
    ).toBe("100");
  });

  it("falls back to the most recent buy/sell price when no snapshot has a price", () => {
    const price = latestHoldingPrice(
      [snap("2026-06", null)],
      [
        trade("buy", "2026-05-01T00:00:00Z", "80"),
        trade("sell", "2026-06-10T00:00:00Z", "95"),
        trade("dividend", "2026-06-20T00:00:00Z", null),
        trade("fee", "2026-06-25T00:00:00Z", "1"),
      ],
      "2026-07",
    );
    expect(price).toBe("95");
  });

  it("is null with nothing to go on", () => {
    expect(latestHoldingPrice(undefined, undefined, "2026-07")).toBeNull();
    expect(
      latestHoldingPrice([], [trade("buy", "2026-05-01T00:00:00Z", null)], "2026-07"),
    ).toBeNull();
  });
});

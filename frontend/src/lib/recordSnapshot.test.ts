import { describe, expect, it } from "vitest";
import {
  asOfInputValue,
  defaultRecordMonth,
  formatRecordMonth,
  lastRecordedBefore,
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

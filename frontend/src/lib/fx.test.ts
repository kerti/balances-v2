import { describe, it, expect } from "vitest";
import { availableDisplayCurrencies, resolveDisplayRate, convert } from "@/lib/fx";
import type { FxRate } from "@/api/types";

// resolveDisplayRate/availableDisplayCurrencies only read currency/year_month/
// rate, so fixtures carry just those (cast past the full wire type).
const rate = (currency: string, yearMonth: string, r: string): FxRate =>
  ({ currency, year_month: `${yearMonth}-01T00:00:00Z`, rate: r }) as FxRate;

describe("availableDisplayCurrencies", () => {
  it("lists distinct currencies minus the reporting currency, sorted", () => {
    const rates = [
      rate("USD", "2026-03", "16000"),
      rate("USD", "2026-04", "16100"),
      rate("SGD", "2026-04", "12000"),
      rate("IDR", "2026-04", "1"), // reporting currency, never stored in practice
    ];
    expect(availableDisplayCurrencies(rates, "IDR")).toEqual(["SGD", "USD"]);
  });

  it("returns empty when only the reporting currency has rates", () => {
    expect(availableDisplayCurrencies([], "IDR")).toEqual([]);
  });
});

describe("resolveDisplayRate", () => {
  const rates = [
    rate("USD", "2026-03", "16000"),
    rate("USD", "2026-05", "16500"),
    rate("SGD", "2026-04", "12000"),
  ];

  it("uses the exact-month rate when present", () => {
    expect(resolveDisplayRate(rates, "USD", "2026-05-01T00:00:00Z")).toEqual({
      rate: 16500,
      rateMonth: "2026-05-01T00:00:00Z",
    });
  });

  it("carries forward the most recent rate <= the month", () => {
    expect(resolveDisplayRate(rates, "USD", "2026-04-01T00:00:00Z")).toEqual({
      rate: 16000,
      rateMonth: "2026-03-01T00:00:00Z",
    });
  });

  it("picks the most recent rate regardless of list order", () => {
    const unsorted = [rate("USD", "2026-05", "16500"), rate("USD", "2026-03", "16000")];
    expect(resolveDisplayRate(unsorted, "USD", "2026-05-01T00:00:00Z")).toEqual({
      rate: 16500,
      rateMonth: "2026-05-01T00:00:00Z",
    });
  });

  it("returns null when no rate exists on or before the month", () => {
    expect(resolveDisplayRate(rates, "USD", "2026-02-01T00:00:00Z")).toBeNull();
  });

  it("returns null for an unknown currency", () => {
    expect(resolveDisplayRate(rates, "EUR", "2026-05-01T00:00:00Z")).toBeNull();
  });

  it("returns null when the stored rate is non-positive or garbage", () => {
    expect(
      resolveDisplayRate([rate("USD", "2026-05", "0")], "USD", "2026-05-01T00:00:00Z"),
    ).toBeNull();
    expect(
      resolveDisplayRate([rate("USD", "2026-05", "oops")], "USD", "2026-05-01T00:00:00Z"),
    ).toBeNull();
  });
});

describe("convert", () => {
  it("divides a reporting-currency amount by the rate", () => {
    expect(convert("1250000000", 16000)).toBe(78125);
  });
});

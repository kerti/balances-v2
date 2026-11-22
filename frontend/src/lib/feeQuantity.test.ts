// covers: INV-FINANCE-42
import { describe, it, expect } from "vitest";
import { deriveFeeQuantity } from "@/lib/feeQuantity";

describe("deriveFeeQuantity", () => {
  it("divides cash by price into units", () => {
    // IDR 50,000 storage fee settled at IDR 1,000,000/g → 0.05 g.
    expect(deriveFeeQuantity("50000", "1000000")).toBe("0.05");
  });

  it("trims the trailing zeros the round introduces", () => {
    expect(deriveFeeQuantity("100", "4")).toBe("25");
    expect(deriveFeeQuantity("1", "2")).toBe("0.5");
  });

  it("rounds to 8 decimal places (DECIMAL(20,8))", () => {
    // 1 / 3 = 0.3333… → 0.33333333
    expect(deriveFeeQuantity("1", "3")).toBe("0.33333333");
  });

  // Exact division, rounded half away from zero to 8 places, zeros trimmed.
  const rows: Array<[string, string, string, string]> = [
    ["a whole result", "100", "4", "25"],
    ["a terminating fraction", "1", "8", "0.125"],
    ["repeating thirds round down", "1", "3", "0.33333333"],
    ["repeating thirds round up", "2", "3", "0.66666667"],
    ["a 9th decimal of 5 rounds up", "1", "512", "0.00195313"],
    ["a 9th decimal below 5 rounds down", "0.000000004", "1", "0"],
    ["a 9th decimal of exactly 5 rounds up", "0.000000005", "1", "0.00000001"],
    ["decimal operands", "1.5", "0.5", "3"],
    ["a float artefact avoided", "0.3", "0.1", "3"],
    ["every digit of a wide quotient", "123456789012.12345678", "1", "123456789012.12345678"],
    ["a wide dividend", "99999999999999.99", "3", "33333333333333.33"],
  ];
  it.each(rows)("%s", (_name, amount, price, want) => {
    expect(deriveFeeQuantity(amount, price)).toBe(want);
  });

  it("returns null when either field is blank", () => {
    expect(deriveFeeQuantity("", "1000000")).toBeNull();
    expect(deriveFeeQuantity("50000", "")).toBeNull();
    expect(deriveFeeQuantity("", "")).toBeNull();
  });

  it("returns null for non-numeric input", () => {
    expect(deriveFeeQuantity("abc", "1000000")).toBeNull();
    expect(deriveFeeQuantity("50000", "xyz")).toBeNull();
  });

  it("returns null for zero or negative values (no division by zero)", () => {
    expect(deriveFeeQuantity("50000", "0")).toBeNull();
    expect(deriveFeeQuantity("0", "1000000")).toBeNull();
    expect(deriveFeeQuantity("-50000", "1000000")).toBeNull();
    expect(deriveFeeQuantity("50000", "-1000000")).toBeNull();
  });
});

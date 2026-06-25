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

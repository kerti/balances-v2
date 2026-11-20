import { describe, expect, it } from "vitest";
import { valueSwing, type Swing } from "@/lib/valueSwing";

// covers: INV-SNAPSHOTS-19
describe("valueSwing", () => {
  const cases: Array<[string, string | null | undefined, string | null, Swing]> = [
    // The slipped digit, both ways, and the exact boundaries (inclusive).
    ["an extra zero", "5000000", "50000000", "up"],
    ["a dropped zero", "5000000", "500000", "down"],
    ["exactly 10x", "100", "1000", "up"],
    ["exactly a tenth", "1000", "100", "down"],
    ["just under 10x", "100", "999.99", null],
    ["just over a tenth", "1000", "100.01", null],
    // Ordinary month-to-month movement.
    ["a small rise", "1000", "1100", null],
    ["unchanged", "1000", "1000", null],
    ["a halving", "1000", "500", null],
    // Decimals are exact, not float: 0.1 * 10 and 0.3 - 0.1 style traps.
    ["decimal prior, exact 10x", "0.1", "1", "up"],
    ["decimal next, exact tenth", "1", "0.1", "down"],
    ["cents just under 10x", "1234.56", "12345.59", null],
    ["cents exactly 10x", "1234.56", "12345.6", "up"],
    ["eight-place quantity scale", "0.00000001", "0.0000001", "up"],
    ["a 20-digit figure keeps every place", "1234567890123456.7891", "12345678901234567.891", "up"],
    // No basis, no question.
    ["no prior (undefined)", undefined, "1000000", null],
    ["no prior (null)", null, "1000000", null],
    ["empty prior", "", "1000000", null],
    ["zero prior", "0", "1000000", null],
    ["zero prior with decimals", "0.00", "5", null],
    ["no new value", "1000", null, null],
    ["empty new value", "1000", "", null],
    ["unreadable new value", "1000", "abc", null],
    // Zero closes a position on purpose.
    ["new value zero", "5000000", "0", null],
    // Negative values compare by magnitude, and a sign change is not a decimal shift.
    ["negative up", "-100", "-1000", "up"],
    ["negative down", "-1000", "-100", "down"],
    ["negative small move", "-100", "-150", null],
    ["sign flip", "100", "-1000", null],
    ["sign flip back", "-100", "1000", null],
  ];

  it.each(cases)("%s", (_name, prior, next, want) => {
    expect(valueSwing(prior, next)).toBe(want);
  });
});

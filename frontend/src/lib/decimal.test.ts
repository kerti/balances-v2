import { describe, it, expect } from "vitest";
import {
  sumDecimals,
  toScaled,
  fromScaled,
  multiplyDecimals,
  subtractDecimals,
  divideDecimals,
} from "@/lib/decimal";

describe("sumDecimals", () => {
  it("is exact where floats drift", () => {
    expect(0.1 + 0.2).not.toBe(0.3);
    expect(sumDecimals(["0.1", "0.2"])).toBe("0.3");
    expect(sumDecimals(["40", "10"])).toBe("50");
    expect(sumDecimals(["0.00000001", "0.00000002"])).toBe("0.00000003");
  });

  it("returns 0 for nothing, null and junk", () => {
    expect(sumDecimals([])).toBe("0");
    expect(sumDecimals([null, undefined, "", "abc", "."])).toBe("0");
  });

  it("handles negatives and a result that crosses zero", () => {
    expect(sumDecimals(["-1.5", "0.5"])).toBe("-1");
    expect(sumDecimals(["-0.25", "0.25"])).toBe("0");
    expect(sumDecimals(["-0.5"])).toBe("-0.5");
  });

  it("keeps full DECIMAL(20,4) magnitudes", () => {
    expect(sumDecimals(["9999999999999999.9999", "0.0001"])).toBe("10000000000000000");
  });

  it("trims trailing zeros of wire decimals", () => {
    expect(sumDecimals(["150.0000", "25.5000"])).toBe("175.5");
  });
});

describe("toScaled / fromScaled", () => {
  it("round-trips", () => {
    expect(fromScaled(toScaled("12.3400")!)).toBe("12.34");
    expect(toScaled("1e3")).toBeNull();
  });
});

// Rows mirror backend/internal/snapshotamount/snapshotamount_test.go: the Go
// helper and this one must print the same string for every row.
// covers: INV-SNAPSHOTS-20
describe("multiplyDecimals", () => {
  const rows: Array<[string, string, string, string]> = [
    ["whole numbers", "40", "110", "4400"],
    ["fractional quantity", "0.5", "9.99", "4.995"],
    ["float artefact avoided", "0.1", "3", "0.3"],
    ["eight-place quantity, four-place price", "1.12345678", "2.1234", "2.3855"],
    ["fifth decimal 5 rounds up", "0.00005", "1", "0.0001"],
    ["fifth decimal below 5 rounds down", "0.00004", "1", "0"],
    ["just above the half rounds up", "1.00005001", "1", "1.0001"],
    ["just below the half rounds down", "1.00004999", "1", "1"],
    ["twelve-place product rounds once", "0.12345678", "0.1234", "0.0152"],
    ["trailing zeros trimmed", "2.50000000", "4.0000", "10"],
    ["20-digit figures", "12345678901234.5", "1000000", "12345678901234500000"],
    [
      "product wider than the column",
      "99999999999999",
      "99999999999999",
      "9999999999999800000000000001",
    ],
    ["zero quantity", "0", "123.4567", "0"],
    ["zero price", "7.5", "0", "0"],
    ["negative rounds away from zero", "-0.00005", "1", "-0.0001"],
    ["negative below the half", "-0.00004", "1", "0"],
    ["negative quantity", "-2", "1.5", "-3"],
  ];

  it.each(rows)("%s", (_name, a, b, want) => {
    expect(multiplyDecimals(a, b)).toBe(want);
  });

  it("is exact where floats drift or go exponential", () => {
    expect(0.1 * 3).not.toBe(0.3);
    expect(multiplyDecimals("0.1", "3")).toBe("0.3");
    expect(multiplyDecimals("12345678901234567890", "1000")).toBe("12345678901234567890000");
  });

  it("returns null for nothing and junk", () => {
    expect(multiplyDecimals("", "3")).toBeNull();
    expect(multiplyDecimals("2", "abc")).toBeNull();
    expect(multiplyDecimals(".", "2")).toBeNull();
    expect(multiplyDecimals("1e3", "2")).toBeNull();
  });
});

describe("subtractDecimals", () => {
  it("is exact where floats drift", () => {
    expect(0.3 - 0.1).not.toBe(0.2);
    expect(subtractDecimals("0.3", "0.1")).toBe("0.2");
    expect(subtractDecimals("1500000.5", "0.25")).toBe("1500000.25");
  });

  it("handles a negative result and zero", () => {
    expect(subtractDecimals("1", "2.5")).toBe("-1.5");
    expect(subtractDecimals("2.5", "2.5")).toBe("0");
  });

  it("keeps DECIMAL(20,4) magnitudes", () => {
    expect(subtractDecimals("9999999999999999.9999", "0.0001")).toBe("9999999999999999.9998");
  });

  it("returns null for nothing and junk", () => {
    expect(subtractDecimals("", "1")).toBeNull();
    expect(subtractDecimals("1", "x")).toBeNull();
  });
});

// covers: INV-FINANCE-42
describe("divideDecimals", () => {
  const rows: Array<[string, string, string, string]> = [
    ["whole", "100", "4", "25"],
    ["trailing zeros trimmed", "1", "2", "0.5"],
    ["repeating rounds down at 8", "1", "3", "0.33333333"],
    ["repeating rounds up at 8", "2", "3", "0.66666667"],
    ["9th decimal 5 rounds up", "0.000000005", "1", "0.00000001"],
    ["9th decimal below 5 rounds down", "0.000000004", "1", "0"],
    ["negative rounds away from zero", "-0.000000005", "1", "-0.00000001"],
    ["negative dividend and divisor", "-1", "-3", "0.33333333"],
    ["mixed signs", "1", "-8", "-0.125"],
    ["zero dividend", "0", "7", "0"],
    ["decimal divisor", "1.5", "0.5", "3"],
    ["every digit kept", "123456789012.12345678", "1", "123456789012.12345678"],
  ];
  it.each(rows)("%s", (_name, a, b, want) => {
    expect(divideDecimals(a, b)).toBe(want);
  });

  it("returns null for a zero divisor, nothing and junk", () => {
    expect(divideDecimals("1", "0")).toBeNull();
    expect(divideDecimals("1", "0.000")).toBeNull();
    expect(divideDecimals("", "3")).toBeNull();
    expect(divideDecimals("2", "abc")).toBeNull();
    expect(divideDecimals("1e3", "2")).toBeNull();
  });
});

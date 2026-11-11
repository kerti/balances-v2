import { describe, it, expect } from "vitest";
import { sumDecimals, toScaled, fromScaled } from "@/lib/decimal";

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

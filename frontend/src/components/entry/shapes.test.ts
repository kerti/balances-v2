import { describe, expect, it } from "vitest";
import { accruedShape, amountOnlyShape, minorUnitsShown, qtyPriceShape } from "./shapes";

// covers: INV-SNAPSHOTS-10
describe("minorUnitsShown", () => {
  it("rounds to the currency's minor units without padding", () => {
    expect(minorUnitsShown("1408210.1634", "IDR")).toBe("1408210.16");
    expect(minorUnitsShown("48056131.79", "IDR")).toBe("48056131.79");
    expect(minorUnitsShown("100.5", "USD")).toBe("100.5");
    expect(minorUnitsShown("12500000", "IDR")).toBe("12500000");
    expect(minorUnitsShown("12500000.0040", "IDR")).toBe("12500000");
  });

  it("shows a 0-decimal currency in whole units", () => {
    expect(minorUnitsShown("1500.4", "JPY")).toBe("1500");
    expect(minorUnitsShown("1500.6", "KRW")).toBe("1501");
  });

  it("keeps zero, a negative and an empty value sensible", () => {
    expect(minorUnitsShown("0", "IDR")).toBe("0");
    expect(minorUnitsShown("0.0000", "USD")).toBe("0");
    expect(minorUnitsShown("-250.1264", "IDR")).toBe("-250.13");
    expect(minorUnitsShown("", "IDR")).toBe("");
  });
});

describe("which fields are money", () => {
  it("flags amounts and accrued interest, never a quantity or a price per unit", () => {
    const flags = (s: { fields: { key: string; money?: boolean }[] }) =>
      Object.fromEntries(s.fields.map((f) => [f.key, f.money === true]));
    expect(flags(amountOnlyShape)).toEqual({ amount: true });
    expect(flags(accruedShape)).toEqual({ amount: true, accrued_interest: true });
    expect(flags(qtyPriceShape)).toEqual({ quantity: false, price_per_unit: false });
  });
});

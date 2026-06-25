import { describe, it, expect } from "vitest";
import {
  formatGoldPurity,
  goldPurityPresetKarat,
  GOLD_PURITY_PRESETS,
} from "@/lib/gold";

describe("formatGoldPurity", () => {
  it("renders pure gold (0.999+) as 24K with the conventional suffix", () => {
    expect(formatGoldPurity("0.9999")).toBe("24K (.999+)");
    expect(formatGoldPurity("0.999")).toBe("24K (.999+)");
    expect(formatGoldPurity("1")).toBe("24K (.999+)");
  });

  it("renders clean karat fractions as whole karats", () => {
    expect(formatGoldPurity("0.9167")).toBe("22K"); // 22/24 ≈ 0.91667
    expect(formatGoldPurity("0.75")).toBe("18K");
    expect(formatGoldPurity("0.5833")).toBe("14K"); // 14/24 ≈ 0.58333
  });

  it("falls through to a percentage when no clean karat fits", () => {
    expect(formatGoldPurity("0.8")).toBe("80.00%");
    expect(formatGoldPurity("0.123")).toBe("12.30%");
  });

  it("returns the raw input for non-numeric or out-of-range values", () => {
    expect(formatGoldPurity("not-a-number")).toBe("not-a-number");
    expect(formatGoldPurity("0")).toBe("0");
    expect(formatGoldPurity("-0.5")).toBe("-0.5");
    expect(formatGoldPurity("1.5")).toBe("1.5");
  });
});

describe("goldPurityPresetKarat", () => {
  it("matches each preset to its karat", () => {
    for (const p of GOLD_PURITY_PRESETS) {
      expect(goldPurityPresetKarat(p.value)).toBe(p.karat);
    }
  });

  it("matches trailing-zero variants of a preset", () => {
    expect(goldPurityPresetKarat("0.75")).toBe(18);
  });

  it("keeps a 0.999 bar off the 0.9999 (24K) preset", () => {
    expect(goldPurityPresetKarat("0.999")).toBeNull();
  });

  it("returns null for off-grid and non-numeric purities", () => {
    expect(goldPurityPresetKarat("0.92")).toBeNull();
    expect(goldPurityPresetKarat("not-a-number")).toBeNull();
  });
});

import { describe, it, expect } from "vitest";
import { maturityInfo, maturityClass } from "@/lib/maturity";

// Fixed reference point so the day-bucket boundaries are deterministic.
const now = new Date(2026, 4, 27); // 2026-05-27, local (matches daysBetween's date-only math)

// daysBetween truncates both ends to date-only, so a plain YYYY-MM-DD string
// landing N days out gives exactly N days regardless of parse-time TZ.
const dateInDays = (days: number): string => {
  const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + days);
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${mm}-${dd}`;
};

describe("maturityInfo", () => {
  it("returns the default state with empty label for an unparseable date", () => {
    expect(maturityInfo("not-a-date", now)).toEqual({
      state: "default",
      label: "",
    });
  });

  it("classifies a date more than 90 days out as default", () => {
    const info = maturityInfo(dateInDays(120), now);
    expect(info.state).toBe("default");
    expect(info.label).toMatch(/^Matures /);
  });

  it("classifies 31–90 days out as approaching", () => {
    expect(maturityInfo(dateInDays(90), now).state).toBe("approaching");
    expect(maturityInfo(dateInDays(31), now).state).toBe("approaching");
  });

  it("classifies 1–30 days out as imminent with a day countdown", () => {
    const info = maturityInfo(dateInDays(18), now);
    expect(info).toEqual({ state: "imminent", label: "Matures in 18 days" });
    expect(maturityInfo(dateInDays(30), now).state).toBe("imminent");
  });

  it('uses "Matures today" at zero days', () => {
    expect(maturityInfo(dateInDays(0), now)).toEqual({
      state: "imminent",
      label: "Matures today",
    });
  });

  it("classifies a past date as matured with a warning prefix", () => {
    const info = maturityInfo(dateInDays(-5), now);
    expect(info.state).toBe("matured");
    expect(info.label).toMatch(/^⚠ Matured /);
  });
});

describe("maturityClass", () => {
  it("maps imminent to amber + bold", () => {
    expect(maturityClass("imminent")).toContain("amber");
    expect(maturityClass("imminent")).toContain("font-semibold");
  });

  it("maps approaching to bold only", () => {
    expect(maturityClass("approaching")).toBe("font-semibold");
  });

  it("maps matured and default to muted", () => {
    expect(maturityClass("matured")).toBe("text-muted-foreground");
    expect(maturityClass("default")).toBe("text-muted-foreground");
  });
});

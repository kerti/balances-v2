import { describe, it, expect } from "vitest";
import { monthRange } from "@/lib/months";

describe("monthRange", () => {
  it("returns a single month when bounds coincide", () => {
    expect(monthRange("2026-03", "2026-03")).toEqual(["2026-03"]);
  });

  it("enumerates consecutive months inclusively", () => {
    expect(monthRange("2026-01", "2026-04")).toEqual([
      "2026-01",
      "2026-02",
      "2026-03",
      "2026-04",
    ]);
  });

  it("fills the gap between non-adjacent bounds", () => {
    expect(monthRange("2026-01", "2026-06")).toEqual([
      "2026-01",
      "2026-02",
      "2026-03",
      "2026-04",
      "2026-05",
      "2026-06",
    ]);
  });

  it("crosses a year boundary", () => {
    expect(monthRange("2025-11", "2026-02")).toEqual([
      "2025-11",
      "2025-12",
      "2026-01",
      "2026-02",
    ]);
  });

  it("accepts the API YYYY-MM-DDTHH year_month shape", () => {
    expect(monthRange("2026-01-01T00:00:00Z", "2026-03-15T12:00:00Z")).toEqual([
      "2026-01",
      "2026-02",
      "2026-03",
    ]);
  });

  it("returns [] when last precedes first", () => {
    expect(monthRange("2026-05", "2026-02")).toEqual([]);
  });

  it("returns [] on a malformed bound", () => {
    expect(monthRange("nope", "2026-02")).toEqual([]);
    expect(monthRange("2026-13", "2026-14")).toEqual([]);
  });
});

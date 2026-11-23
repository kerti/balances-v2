import { describe, expect, it } from "vitest";
import { TAG_SWATCHES, displayTagColor } from "./tagColors";

describe("tag palette (ADR-0054: red is for debt and negatives only)", () => {
  it("offers no red swatch", () => {
    expect(TAG_SWATCHES).not.toContain("#ef4444");
  });

  it("draws a stored legacy red tag in a palette colour instead", () => {
    const shown = displayTagColor("#EF4444");
    expect(shown).not.toBe("#EF4444");
    expect(TAG_SWATCHES).toContain(shown);
  });

  it("leaves every other colour alone", () => {
    for (const c of TAG_SWATCHES) expect(displayTagColor(c)).toBe(c);
  });
});

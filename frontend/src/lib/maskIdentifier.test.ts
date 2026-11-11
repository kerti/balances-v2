import { describe, expect, it } from "vitest";
import { maskIdentifier } from "@/lib/maskIdentifier";

describe("maskIdentifier", () => {
  it("keeps only the last four characters", () => {
    expect(maskIdentifier("1234567890")).toBe("•••• 7890");
    expect(maskIdentifier("12345")).toBe("•••• 2345");
  });

  it("trims before masking", () => {
    expect(maskIdentifier("  1234567890 ")).toBe("•••• 7890");
  });

  it("masks a short value entirely rather than revealing it", () => {
    expect(maskIdentifier("1234")).toBe("••••");
    expect(maskIdentifier("111")).toBe("•••");
  });

  it("returns blank for a blank value", () => {
    expect(maskIdentifier("")).toBe("");
    expect(maskIdentifier("   ")).toBe("");
  });
});

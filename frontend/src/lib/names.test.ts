import { describe, it, expect } from "vitest";
import { preferredName, initials } from "@/lib/names";

describe("preferredName", () => {
  it("returns the nickname when set", () => {
    expect(preferredName({ nickname: "Ali", display_name: "Alice Anderson" })).toBe("Ali");
  });

  it("falls back to display_name when nickname is null", () => {
    expect(preferredName({ nickname: null, display_name: "Alice" })).toBe("Alice");
  });

  it("falls back to display_name when nickname is undefined", () => {
    expect(preferredName({ display_name: "Alice" })).toBe("Alice");
  });

  it("treats blank/whitespace nickname as unset", () => {
    expect(preferredName({ nickname: "", display_name: "Alice" })).toBe("Alice");
    expect(preferredName({ nickname: "   ", display_name: "Alice" })).toBe("Alice");
  });

  it("trims a set nickname", () => {
    expect(preferredName({ nickname: "  Ali  ", display_name: "Alice" })).toBe("Ali");
  });
});

describe("initials", () => {
  it("uses first + last word initials for multi-word names", () => {
    expect(initials("Alice Tan")).toBe("AT");
    expect(initials("Alice Mary Tan")).toBe("AT");
  });

  it("uses the single initial for one-word names", () => {
    expect(initials("Alice")).toBe("A");
  });

  it("uppercases", () => {
    expect(initials("alice tan")).toBe("AT");
  });

  it("collapses extra whitespace", () => {
    expect(initials("  Alice   Tan  ")).toBe("AT");
  });

  it('returns "?" for an empty or blank name', () => {
    expect(initials("")).toBe("?");
    expect(initials("   ")).toBe("?");
  });
});

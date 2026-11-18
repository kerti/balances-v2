import { describe, expect, it } from "vitest";
import { isLeafActive, isSectionActive } from "@/components/shell/navActive";

const assets = {
  to: "/assets",
  children: [{ to: "/assets/bank-accounts" }, { to: "/assets/properties" }],
};
const dashboard = { to: "/" };

describe("sidebar active state", () => {
  // covers: INV-PRESENTATION-32
  it("highlights the section on a page beneath it that no child claims", () => {
    expect(isSectionActive("/assets", assets)).toBe(true);
    expect(isSectionActive("/assets/enter", assets)).toBe(true);
  });

  // covers: INV-PRESENTATION-32
  it("leaves the highlight to the child when one matches", () => {
    expect(isSectionActive("/assets/bank-accounts/abc", assets)).toBe(false);
    expect(isLeafActive("/assets/bank-accounts/abc", "/assets/bank-accounts")).toBe(true);
    expect(isSectionActive("/assets/bank-accounts", assets)).toBe(false);
  });

  it("does not highlight a section for an unrelated path", () => {
    expect(isSectionActive("/assetsfoo", assets)).toBe(false);
    expect(isSectionActive("/income", assets)).toBe(false);
  });

  it("keeps a childless section prefix-active and the dashboard exact-only", () => {
    expect(isSectionActive("/receivables/enter", { to: "/receivables" })).toBe(true);
    expect(isSectionActive("/", dashboard)).toBe(true);
    expect(isSectionActive("/income", dashboard)).toBe(false);
  });
});

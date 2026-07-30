import { describe, expect, it } from "vitest";
import { pageWindow } from "./pageWindow";

// covers: INV-PRESENTATION-08
describe("pageWindow", () => {
  it("renders every page while they all fit", () => {
    expect(pageWindow(1, 6, 1)).toEqual([1, 2, 3, 4, 5, 6]);
    expect(pageWindow(1, 5, 0)).toEqual([1, 2, 3, 4, 5]);
  });

  it("truncates once they do not", () => {
    expect(pageWindow(10, 40, 1)).toEqual([1, "ellipsis", 9, 10, 11, "ellipsis", 40]);
    expect(pageWindow(10, 40, 0)).toEqual([1, "ellipsis", 10, "ellipsis", 40]);
  });

  // The point of the constant count: the control keeps the same width on every
  // page, so paging through does not shift the row (or its neighbours) around.
  it.each([0, 1, 2])("holds a constant item count at siblings=%i", (siblings) => {
    const slots = siblings * 2 + 5;
    const widths = new Set(
      Array.from({ length: 40 }, (_, i) => pageWindow(i + 1, 40, siblings).length),
    );
    expect([...widths]).toEqual([slots]);
  });

  it("keeps the first and last page reachable from anywhere", () => {
    for (const page of [1, 2, 7, 20, 39, 40]) {
      const items = pageWindow(page, 40, 0);
      expect(items[0]).toBe(1);
      expect(items[items.length - 1]).toBe(40);
      expect(items).toContain(page);
    }
  });

  it("extends the run of numbers at each end instead of shrinking", () => {
    expect(pageWindow(1, 40, 1)).toEqual([1, 2, 3, 4, 5, "ellipsis", 40]);
    expect(pageWindow(40, 40, 1)).toEqual([1, "ellipsis", 36, 37, 38, 39, 40]);
  });

  // An ellipsis covering one page costs the slot the page would have taken and
  // hides nothing, so the page wins.
  it("spells out a gap of exactly one page", () => {
    // Page 4 of 40: only page 2 sits between the first page and the window.
    expect(pageWindow(4, 40, 1)).toEqual([1, 2, 3, 4, 5, "ellipsis", 40]);
    // Page 37 of 40: only page 39 sits between the window and the last page.
    expect(pageWindow(37, 40, 1)).toEqual([1, "ellipsis", 36, 37, 38, 39, 40]);
    // Two hidden pages on each side is a real gap, and stays an ellipsis.
    expect(pageWindow(5, 9, 1)).toEqual([1, "ellipsis", 4, 5, 6, "ellipsis", 9]);
  });

  it("clamps a page outside the range rather than rendering a gap", () => {
    expect(pageWindow(0, 40, 0)).toEqual(pageWindow(1, 40, 0));
    expect(pageWindow(99, 40, 0)).toEqual(pageWindow(40, 40, 0));
  });

  it("renders nothing when there are no pages", () => {
    expect(pageWindow(1, 0)).toEqual([]);
  });
});

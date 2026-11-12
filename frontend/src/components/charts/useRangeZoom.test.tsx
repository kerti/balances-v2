import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useRangeZoom } from "./useRangeZoom";

beforeEach(() => {
  localStorage.clear();
  window.innerWidth = 1280;
});
afterEach(() => localStorage.clear());

// covers: INV-PRESENTATION-29
describe("useRangeZoom", () => {
  it("offers chips by history length", () => {
    expect(renderHook(() => useRangeZoom(8)).result.current.chips).toEqual([]);
    expect(renderHook(() => useRangeZoom(30)).result.current.chips).toEqual(["1Y", "All"]);
  });

  it("shares the picked chip with a chart mounted afterwards", () => {
    const first = renderHook(() => useRangeZoom(100));
    act(() => first.result.current.pickPreset("3Y"));
    expect(first.result.current.pressed).toBe("3Y");
    expect(first.result.current.window).toEqual({ start: 64, end: 99 });

    const second = renderHook(() => useRangeZoom(100));
    expect(second.result.current.pressed).toBe("3Y");
    expect(second.result.current.window).toEqual({ start: 64, end: 99 });
  });

  it("does not remember a dragged window", () => {
    const first = renderHook(() => useRangeZoom(100));
    act(() => first.result.current.onWindowChange({ start: 10, end: 20 }));
    expect(first.result.current.pressed).toBeNull();
    expect(renderHook(() => useRangeZoom(100)).result.current.pressed).toBe("5Y");
  });

  it("shows All for a hidden stored chip without overwriting the choice", () => {
    localStorage.setItem("balances.dashboard.chartRange", "5Y");
    const short = renderHook(() => useRangeZoom(20));
    expect(short.result.current.pressed).toBe("All");
    expect(short.result.current.window).toEqual({ start: 0, end: 19 });
    expect(localStorage.getItem("balances.dashboard.chartRange")).toBe("5Y");
    expect(renderHook(() => useRangeZoom(100)).result.current.pressed).toBe("5Y");
  });

  it("shows the Brush from 3 months on desktop only", () => {
    expect(renderHook(() => useRangeZoom(2)).result.current.showBrush).toBe(false);
    expect(renderHook(() => useRangeZoom(3)).result.current.showBrush).toBe(true);
    window.innerWidth = 390;
    expect(renderHook(() => useRangeZoom(30)).result.current.showBrush).toBe(false);
  });
});

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  DEFAULT_RANGE_PRESET,
  RANGE_STORAGE_KEY,
  loadRangePreset,
  presetWindow,
  saveRangePreset,
  effectivePreset,
  sliceWindow,
  visiblePresets,
  shiftToInclude,
} from "./chartRange";

describe("presetWindow", () => {
  it("takes the most recent N months", () => {
    expect(presetWindow("1Y", 132)).toEqual({ start: 120, end: 131 });
    expect(presetWindow("3Y", 132)).toEqual({ start: 96, end: 131 });
    expect(presetWindow("5Y", 132)).toEqual({ start: 72, end: 131 });
    expect(presetWindow("All", 132)).toEqual({ start: 0, end: 131 });
  });

  it("clamps to the available history", () => {
    expect(presetWindow("5Y", 20)).toEqual({ start: 0, end: 19 });
    expect(presetWindow("1Y", 1)).toEqual({ start: 0, end: 0 });
  });
});

describe("shiftToInclude", () => {
  it("leaves a window that already contains the month alone", () => {
    const w = { start: 72, end: 131 };
    expect(shiftToInclude(w, 100, 132)).toBe(w);
  });

  it("slides earlier, keeping the width", () => {
    expect(shiftToInclude({ start: 120, end: 131 }, 90, 132)).toEqual({ start: 90, end: 101 });
  });

  it("slides later, keeping the width", () => {
    expect(shiftToInclude({ start: 0, end: 11 }, 40, 132)).toEqual({ start: 29, end: 40 });
  });

  it("ignores an index outside the data", () => {
    const w = { start: 0, end: 11 };
    expect(shiftToInclude(w, -1, 132)).toBe(w);
    expect(shiftToInclude(w, 500, 132)).toBe(w);
  });
});

describe("range persistence", () => {
  function fakeStorage(init: Record<string, string> = {}) {
    const m = new Map(Object.entries(init));
    return {
      getItem: (k: string) => m.get(k) ?? null,
      setItem: (k: string, v: string) => void m.set(k, v),
    };
  }

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("defaults to 5Y when nothing is stored", () => {
    vi.stubGlobal("localStorage", fakeStorage());
    expect(loadRangePreset()).toBe(DEFAULT_RANGE_PRESET);
    expect(DEFAULT_RANGE_PRESET).toBe("5Y");
  });

  it("round-trips a saved choice", () => {
    vi.stubGlobal("localStorage", fakeStorage());
    saveRangePreset("3Y");
    expect(loadRangePreset()).toBe("3Y");
  });

  it("falls back to the default on a garbage value", () => {
    vi.stubGlobal("localStorage", fakeStorage({ [RANGE_STORAGE_KEY]: "10Y" }));
    expect(loadRangePreset()).toBe("5Y");
  });

  it("survives storage throwing on read and write", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
    });
    expect(loadRangePreset()).toBe("5Y");
    expect(() => saveRangePreset("1Y")).not.toThrow();
  });
});

// covers: INV-PRESENTATION-29
describe("visiblePresets", () => {
  it("shows no chip row at 12 months or fewer", () => {
    expect(visiblePresets(0)).toEqual([]);
    expect(visiblePresets(8)).toEqual([]);
    expect(visiblePresets(12)).toEqual([]);
  });

  it("shows a chip only when the history is longer than it covers; All always", () => {
    expect(visiblePresets(13)).toEqual(["1Y", "All"]);
    expect(visiblePresets(30)).toEqual(["1Y", "All"]);
    expect(visiblePresets(36)).toEqual(["1Y", "All"]);
    expect(visiblePresets(37)).toEqual(["1Y", "3Y", "All"]);
    expect(visiblePresets(60)).toEqual(["1Y", "3Y", "All"]);
    expect(visiblePresets(61)).toEqual(["1Y", "3Y", "5Y", "All"]);
  });
});

// covers: INV-PRESENTATION-29
describe("effectivePreset", () => {
  it("keeps a stored chip that is visible", () => {
    expect(effectivePreset("3Y", 50)).toBe("3Y");
  });

  it("falls back to All when the stored chip is hidden for a short history", () => {
    expect(effectivePreset("5Y", 20)).toBe("All");
    expect(effectivePreset("1Y", 8)).toBe("All");
  });
});

describe("sliceWindow", () => {
  const rows = ["a", "b", "c", "d", "e"];

  it("returns only the rows inside the window", () => {
    expect(sliceWindow(rows, { start: 1, end: 3 })).toEqual(["b", "c", "d"]);
  });

  it("clamps a window past the end and passes everything without one", () => {
    expect(sliceWindow(rows, { start: 3, end: 40 })).toEqual(["d", "e"]);
    expect(sliceWindow(rows, undefined)).toEqual(rows);
  });
});

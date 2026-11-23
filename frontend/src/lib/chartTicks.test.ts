import { describe, expect, it } from "vitest";
import { MIN_TICK_GAP_PX, TICK_STEPS, pickTickIndices } from "./chartTicks";

// covers: INV-PRESENTATION-28
describe("pickTickIndices", () => {
  // Plot widths at 390 / 768 / 1440 viewports: card width minus the y-axis and margins.
  const widths = [250, 640, 1100];

  it("returns nothing for an empty series", () => {
    expect(pickTickIndices(0, 600)).toEqual([]);
  });

  it("always starts at the first month, at every width and length", () => {
    for (const w of [0, 100, ...widths]) {
      for (const n of [1, 2, 7, 13, 26, 60, 187]) {
        expect(pickTickIndices(n, w)[0]).toBe(0);
      }
    }
  });

  it("labels every month when they all fit", () => {
    expect(pickTickIndices(6, 640)).toEqual([0, 1, 2, 3, 4, 5]);
  });

  it("never fits more labels than the width allows, evenly spaced", () => {
    for (const w of widths) {
      for (const n of [13, 26, 60, 187]) {
        const idx = pickTickIndices(n, w);
        expect(idx.length).toBeLessThanOrEqual(Math.max(1, Math.floor(w / MIN_TICK_GAP_PX)));
        const step = idx[1] - idx[0];
        // Adjacent labels sit `step` slots apart; each slot is w/(n-1) px.
        expect(step * (w / (n - 1))).toBeGreaterThanOrEqual(MIN_TICK_GAP_PX * 0.8);
        idx.slice(1).forEach((v, i) => expect(v - idx[i]).toBe(step));
      }
    }
  });

  it("snaps the step to a calendar-friendly month count", () => {
    const allowed: readonly number[] = TICK_STEPS;
    for (const w of [120, 250, 400, 640, 900, 1100]) {
      for (const n of [3, 8, 13, 26, 40, 60, 97, 187, 300]) {
        const idx = pickTickIndices(n, w);
        if (idx.length < 2) continue;
        const step = idx[1] - idx[0];
        expect(allowed.includes(step) || step % 60 === 0).toBe(true);
      }
    }
  });

  it("never lands on an every-4 or every-5 month cadence", () => {
    // 20 months in ~280px: the raw step would be 4 (every 4 months); it snaps to 6.
    expect(pickTickIndices(20, 280)).toEqual([0, 6, 12, 18]);
    // 31 months in 300px: 5 labels fit, raw step 7, snaps to 12.
    expect(pickTickIndices(31, 300)).toEqual([0, 12, 24]);
    // 22 months in 220px: 3 labels fit, raw step 8, snaps to 12.
    expect(pickTickIndices(22, 220)).toEqual([0, 12]);
  });

  it("thins a 26-month series more on narrower widths", () => {
    expect(pickTickIndices(26, 250).length).toBeLessThan(pickTickIndices(26, 640).length);
    expect(pickTickIndices(26, 640).length).toBeLessThan(pickTickIndices(26, 1100).length);
  });
});

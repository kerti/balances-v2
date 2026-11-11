import { describe, expect, it } from "vitest";
import { fitYDomain } from "./chartDomain";

describe("fitYDomain", () => {
  it("fits a line to its data instead of starting at zero (4%/yr growth stays visible)", () => {
    const values = [100, 104, 108.16, 112.49, 116.99];
    const { domain, ticks } = fitYDomain(values);
    expect(domain[0]).toBeGreaterThan(0);
    expect(domain[0]).toBeLessThanOrEqual(100);
    expect(domain[1]).toBeGreaterThanOrEqual(116.99);
    // The data fills most of the axis rather than a sliver of it.
    expect((116.99 - 100) / (domain[1] - domain[0])).toBeGreaterThan(0.6);
    // The lowest tick is the real (snapped) lower bound.
    expect(ticks[0]).toBe(domain[0]);
    expect(ticks[ticks.length - 1]).toBe(domain[1]);
  });

  it("does not snap a series near zero down to zero (12m–47m starts at 10m)", () => {
    const { domain, ticks } = fitYDomain([12e6, 20e6, 33e6, 47e6]);
    expect(domain).toEqual([10e6, 50e6]);
    expect(ticks[0]).toBe(10e6);
  });

  it("snaps to evenly spaced round ticks", () => {
    const { ticks } = fitYDomain([1_020_000, 1_180_000]);
    const step = ticks[1] - ticks[0];
    for (let i = 1; i < ticks.length; i++) expect(ticks[i] - ticks[i - 1]).toBeCloseTo(step, 6);
    for (const t of ticks) expect(t % step).toBeCloseTo(0, 6);
  });

  it("keeps zero for stacked charts", () => {
    const { domain, ticks } = fitYDomain([500, 520, 540], { includeZero: true });
    expect(domain[0]).toBe(0);
    expect(ticks[0]).toBe(0);
    expect(domain[1]).toBeGreaterThanOrEqual(540);
  });

  it("shows the zero line when a series crosses zero", () => {
    const { domain, ticks, crossesZero } = fitYDomain([-300, -50, 120, 400]);
    expect(crossesZero).toBe(true);
    expect(domain[0]).toBeLessThan(0);
    expect(domain[1]).toBeGreaterThan(0);
    expect(ticks).toContain(0);
  });

  it("does not pad a never-negative series below zero", () => {
    const { domain } = fitYDomain([0.5, 1, 2]);
    expect(domain[0]).toBeGreaterThanOrEqual(0);
  });

  it("gives a flat series a non-degenerate domain", () => {
    const { domain } = fitYDomain([250, 250, 250]);
    expect(domain[1]).toBeGreaterThan(domain[0]);
    expect(domain[0]).toBeLessThanOrEqual(250);
    expect(domain[1]).toBeGreaterThanOrEqual(250);
  });

  it("ignores non-finite values and survives empty input", () => {
    expect(fitYDomain([]).domain).toEqual([0, 1]);
    expect(fitYDomain([NaN, 10, 20]).domain[1]).toBeGreaterThanOrEqual(20);
  });
});

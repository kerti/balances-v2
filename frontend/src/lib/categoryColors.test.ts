/// <reference types="node" />
// covers: INV-PRESENTATION-22
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import {
  ASSET_CATEGORY_COLORS,
  INVESTMENT_CATEGORY_COLORS,
  LIABILITY_CATEGORY_COLORS,
  RISK_PROFILE_COLORS,
} from "./categoryColors";

// The css `?raw` import is empty under vitest (css processing is off), so read the file.
const css = readFileSync(`${import.meta.dirname}/../index.css`, "utf8");

function block(selector: string): string {
  const start = css.indexOf(`\n${selector} {`);
  expect(start).toBeGreaterThan(-1);
  return css.slice(start, css.indexOf("\n}", start));
}

function oklchHue(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  const a = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const bb = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  return ((Math.atan2(bb, a) * 180) / Math.PI + 360) % 360;
}

// Red family around --negative (hue 22-27) and green family around
// --positive (hue 155), with a margin.
const isRedOrGreen = (h: number) => h >= 345 || h <= 45 || (h >= 110 && h <= 180);

const allVars = [
  ...Object.values(ASSET_CATEGORY_COLORS),
  ...Object.values(LIABILITY_CATEGORY_COLORS),
  ...Object.values(INVESTMENT_CATEGORY_COLORS),
].map((v) => v.replace(/^var\((.*)\)$/, "$1"));
const riskVars = Object.values(RISK_PROFILE_COLORS).map((v) => v.replace(/^var\((.*)\)$/, "$1"));

describe("category colours", () => {
  it("every category resolves to a --cat-* token", () => {
    expect(allVars).toHaveLength(10);
    for (const v of allVars) expect(v).toMatch(/^--cat-/);
  });

  it.each([
    [":root", "light"],
    [".dark", "dark"],
  ])("no category is a red or green hue in the %s theme (%s)", (selector) => {
    const body = block(selector);
    for (const v of [...allVars, ...riskVars]) {
      const m = body.match(new RegExp(`${v}:\\s*(#[0-9a-fA-F]{6})`));
      expect(m, `${v} defined in ${selector}`).not.toBeNull();
      expect(isRedOrGreen(oklchHue(m![1])), `${v} ${m![1]}`).toBe(false);
    }
  });
});

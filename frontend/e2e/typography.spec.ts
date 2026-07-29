import { test, expect } from "@playwright/test";

// The app paints in its bundled typeface (#565 / INV-PRESENTATION-09).
//
// `frontend/src/index.css` used to set the app font twice and the two
// disagreed: `body { font-family: system-ui, ... }` beat the
// `html { @apply font-sans }` base layer for everything inside `body` — which
// is the entire app — so `--font-sans: "Geist Variable"` only reached elements
// that carried `font-sans` explicitly. Net effect: the `@fontsource-variable/
// geist` woff2 was fetched on every cold load and then never painted.
//
// Guarded here rather than in vitest because jsdom has neither a font stack nor
// a cascade that resolves `@apply`, so the regression (someone re-declares
// `font-family` on `body`, or on any wrapper above the app tree) is invisible
// to a unit test. It is also not purely cosmetic: `system-ui` resolves to a
// different physical font per platform, so while it held, every text-width
// assertion in the suite was platform-dependent — the #562 settings copy
// measured 268.6px on macOS and 301px on the Linux runner for the same string.
// The backend's PDF renderer already embeds Geist (`reports/pdf/render.go`), so
// this is also what keeps an exported report and the screen it came from the
// same typeface.
//
// Read-only: it measures the rendered shell and writes nothing, so there is no
// seed or cleanup.
//
// covers: INV-PRESENTATION-09
test(
  "the app renders in the bundled Geist, not the platform system font",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    // The first family on `body` is the bundled face. Asserted on `body` rather
    // than `html` because `html` is where the rule lives — `body` is the first
    // node that inherits it, and the exact node the old override sat on.
    const stack = await page.evaluate(() => getComputedStyle(document.body).fontFamily);
    expect(
      stack
        .split(",")[0]
        .trim()
        .replace(/^["']|["']$/g, ""),
    ).toBe("Geist Variable");

    // ...and that family actually resolves to a distinct loaded face rather than
    // silently substituting. `document.fonts.check()` alone cannot tell the two
    // apart — it returns true when the face is loaded *and* when no matching
    // `@font-face` exists at all, because an absent family falls back to a system
    // font that needs no loading (it reported `true` throughout the #565 bug).
    // Measuring the same string in Geist and in the generic fallback separates
    // them: if Geist were not painting, both runs would resolve to the same
    // physical font and the widths would be identical.
    const metrics = await page.evaluate(async () => {
      await document.fonts.ready;
      const ctx = document.createElement("canvas").getContext("2d")!;
      const sample = "The quick brown fox jumps over the lazy dog 0123456789";
      ctx.font = '16px "Geist Variable"';
      const geist = ctx.measureText(sample).width;
      ctx.font = "16px sans-serif";
      return { geist, fallback: ctx.measureText(sample).width };
    });
    expect(metrics.geist).toBeGreaterThan(0);
    expect(metrics.geist).not.toBeCloseTo(metrics.fallback, 1);
  },
);

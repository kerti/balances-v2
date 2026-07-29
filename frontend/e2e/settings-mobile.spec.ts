import { test, expect } from "@playwright/test";

// Settings-home mobile a11y floor (ADR-0050 / INV-PRESENTATION-08). Unlike the
// rate subpages (#511) this surface has no renderer split — it is a stack of
// cards that already reflows — so what needed guarding was the tap floor, which
// the per-callsite sweeps (#507–#542) never reached here: the Settings home page
// had no mobile spec at all, so `make qa-strict` stayed green while three
// selects, four inputs, a checkbox row, two text links and every button in the
// Data section sat between 20px and 36px on a phone.
//
// The floor now comes from the shared primitives (`Button` text sizes and
// `Input` carry `max-md:min-h-11`), so this spec deliberately measures a
// *representative sample across control kinds* rather than one control: a
// primitive-sourced button, a primitive-sourced input, a hand-rolled <select>,
// a checkbox row, and a text link. If the primitive floor regresses, the button
// and input assertions fail together — that is the signal.
//
// Read-only: it measures the rendered settings surface and writes nothing, so
// there is no seed or cleanup.
//
// At <768px the shell collapses the sidebar (ADR-0025), so navigate by URL.

const FLOOR = 44;

// covers: INV-PRESENTATION-08
test(
  "settings home holds the mobile a11y floor across every control kind at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/settings");

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    // No horizontal page scroll — the card stack fits the phone width.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // One representative control per kind. The detail pages are taller than the
    // viewport, so scroll each into view before measuring (a control parked
    // outside the viewport still reports a box, but keeping this consistent with
    // the other mobile specs avoids surprises on lazily-sized rows).
    const samples: Record<string, ReturnType<typeof page.locator>> = {
      // Primitive-sourced button (Button size="default").
      "erase open button": page.getByTestId("erase-open-button"),
      "backup export button": page.getByTestId("backup-export-button"),
      // Primitive-sourced input.
      "reporting currency input": page.locator("#reporting-currency"),
      "nickname input": page.locator("#nickname"),
      // Hand-rolled <select> — not primitive-sourced, floored at the callsite.
      "language select": page.getByTestId("settings-language-select"),
      "carryover date select": page.getByTestId("settings-carryover-date-select"),
      // Checkbox row: the 16px box is the affordance, the label is the hit area.
      "multi-currency toggle row": page.locator("label").filter({
        has: page.locator("input[type=checkbox]"),
      }),
      // Text link out to the inflation-rates subpage.
      "manage inflation rates link": page.locator('a[href="/settings/inflation-rates"]'),
    };

    for (const [name, locator] of Object.entries(samples)) {
      const target = locator.first();
      await target.scrollIntoViewIfNeeded();
      const box = await target.boundingBox();
      expect(box, `${name} should render`).not.toBeNull();
      expect(box!.height, `${name} should clear the ${FLOOR}px tap floor`).toBeGreaterThanOrEqual(
        FLOOR,
      );
    }
  },
);

// covers: INV-PRESENTATION-08
test(
  "settings home keeps the desktop control density at 1280px",
  { tag: "@smoke" },
  async ({ page }) => {
    // The floor is phone-only (`max-md:`). Without this, "raise everything to
    // 44px" would silently pad the desktop forms out — the regression the
    // `md:min-h-0` half of the old per-callsite idiom existed to prevent.
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/settings");

    const button = page.getByTestId("erase-open-button");
    await expect(button).toBeVisible();
    const buttonBox = await button.boundingBox();
    expect(buttonBox).not.toBeNull();
    expect(buttonBox!.height).toBeLessThan(FLOOR);

    const input = page.locator("#reporting-currency");
    const inputBox = await input.boundingBox();
    expect(inputBox).not.toBeNull();
    expect(inputBox!.height).toBeLessThan(FLOOR);
  },
);

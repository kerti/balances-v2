import { test, expect, type Locator } from "@playwright/test";

// Settings-home mobile a11y floor (ADR-0050 / INV-PRESENTATION-08). Unlike the
// rate subpages (#511) this surface has no renderer split — it is a stack of
// cards that already reflows — so what needed guarding was the tap floor, which
// the per-callsite sweeps (#507–#542) never reached here: the Settings home page
// had no mobile spec at all, so `make qa-strict` stayed green while three
// selects, four inputs, a checkbox row, two text links and every button in the
// Data section sat between 20px and 36px on a phone.
//
// The floor now comes from the shared primitives (`Button` text sizes, `Input`
// and — since #541 — `Select` carry `max-md:min-h-11`), so this spec
// deliberately measures a *representative sample across control kinds* rather
// than one control: a primitive-sourced button, input and select, a checkbox
// row, and a text link. If the primitive floor regresses, those assertions fail
// together — that is the signal.
//
// Read-only: it measures the rendered settings surface and writes nothing, so
// there is no seed or cleanup.
//
// At <768px the shell collapses the sidebar (ADR-0025), so navigate by URL.

const FLOOR = 44;

// Chromium draws the native <select> dropdown arrow inside the control's
// content box, so the text run gets less room than `width - padding - border`.
// 20px is a deliberately generous allowance for it — the point of the
// truncation assertion is the width token, not arrow metrology.
const SELECT_ARROW = 20;

// The longest carry-over option in each shipped locale, as rendered at 390px.
// Hardcoded rather than read from `src/locales/*/settings.json`: the e2e specs
// don't import app source, and driving the language select to reach the id-ID
// copy would make this read-only spec mutate the session user (it PATCHes
// `locale`). The `toContain` check below is the drift tripwire — reword the
// en-GB option and this fails, which is the prompt to re-measure the id-ID
// sibling alongside it.
//
// #562 assumed full width alone would clear the truncation and no copy change
// was needed. Measured, it did not: the old "End of the month after the last
// snapshot" renders 291.5px against 284px of text box, so the en-GB option was
// shortened to fit with margin. The estimate was off by more than the arrow
// allowance — hence this test measures rather than eyeballs.
const LONGEST_CARRYOVER_OPTION: Record<string, string> = {
  "en-GB": "End of month after last snapshot",
  "id-ID": "Akhir bulan setelah snapshot terakhir",
};

// Geometry of one control relative to the content box of the card it sits in.
// Measured against the card rather than a pixel constant so these assertions
// survive a change to the shell or card padding.
async function controlMetrics(control: Locator) {
  return control.evaluate((el) => {
    const content = el.closest("[data-slot=card-content]");
    if (!content) throw new Error("control is not inside a CardContent");
    const cardStyle = getComputedStyle(content);
    const cardBox = content.getBoundingClientRect();
    const style = getComputedStyle(el);
    const box = el.getBoundingClientRect();
    return {
      width: box.width,
      right: box.right,
      cardInnerWidth:
        cardBox.width - parseFloat(cardStyle.paddingLeft) - parseFloat(cardStyle.paddingRight),
      cardInnerRight: cardBox.right - parseFloat(cardStyle.paddingRight),
      // Room left for text once padding, borders and the arrow are removed.
      textWidth:
        box.width -
        parseFloat(style.paddingLeft) -
        parseFloat(style.paddingRight) -
        parseFloat(style.borderLeftWidth) -
        parseFloat(style.borderRightWidth),
      font: `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`,
    };
  });
}

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
      // Primitive-sourced select. These were hand-rolled and floored at the
      // callsite until #541 added `ui/select.tsx`; they now inherit the floor.
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

// Width tokens (#562). The floor test above guards how tall a control is; this
// guards how wide. Both symptoms the tokens fix are horizontal: the old
// per-callsite `w-28` / `w-56` / `w-72` left up to 138px of a 326px card dead,
// and clipped the longest carry-over option.
// covers: INV-PRESENTATION-08
test(
  "settings home controls follow the two semantic width tokens at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/settings");

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    // `full`, unpaired: the control is the whole row, so it reaches the card's
    // inner width exactly.
    for (const [name, testId] of [
      ["language select", "settings-language-select"],
      ["theme select", "settings-theme-select"],
      ["carryover date select", "settings-carryover-date-select"],
    ] as const) {
      const control = page.getByTestId(testId);
      await control.scrollIntoViewIfNeeded();
      const m = await controlMetrics(control);
      expect(m.width, `${name} should fill the card width`).toBeGreaterThanOrEqual(
        m.cardInnerWidth - 1,
      );
    }

    // `full`, paired with a Save: the control takes the rest of the row and the
    // button keeps its natural width, which lands it flush against the card's
    // right edge.
    for (const [name, selector] of [
      ["nickname", "#nickname"],
      ["household name", "#household-name"],
    ] as const) {
      const control = page.locator(selector);
      await control.scrollIntoViewIfNeeded();
      const m = await controlMetrics(control);
      expect(m.width, `${name} should take most of the row`).toBeGreaterThan(m.cardInnerWidth / 2);
      expect(m.width, `${name} should leave the Save at natural width`).toBeLessThan(
        m.cardInnerWidth,
      );
      const save = page.locator(`[data-slot=card-content]:has(${selector})`).getByRole("button");
      const box = (await save.boundingBox())!;
      expect(box.x + box.width, `${name} Save should sit flush right`).toBeGreaterThanOrEqual(
        m.cardInnerRight - 1,
      );
    }

    // `narrow`: content-sized, not container-sized — a 326px box for a
    // three-letter currency code communicates the wrong expected input. The
    // Save still sits flush right, so the row reads the same as a `full` one.
    for (const [name, selector] of [
      ["reporting currency", "#reporting-currency"],
      ["assumed inflation", "#assumed-inflation"],
    ] as const) {
      const control = page.locator(selector);
      await control.scrollIntoViewIfNeeded();
      const m = await controlMetrics(control);
      expect(m.width, `${name} should stay narrow`).toBeLessThan(m.cardInnerWidth / 2);
      const save = page.locator(`[data-slot=card-content]:has(${selector})`).getByRole("button");
      const box = (await save.boundingBox())!;
      expect(box.x + box.width, `${name} Save should sit flush right`).toBeGreaterThanOrEqual(
        m.cardInnerRight - 1,
      );
    }

    // No clipped option. `w-72` (288px) left ~248px of text box; full width
    // gives 304px, less the arrow. Both are measured against the 16px
    // `text-base` the Select primitive forces below 768px (iOS Safari zooms on a
    // smaller focused control), which is what made the old copy overflow.
    // Per-locale rather than a single max so a failure names the locale that
    // needs the shorter string.
    const carryover = page.getByTestId("settings-carryover-date-select");
    await carryover.scrollIntoViewIfNeeded();
    const carryoverMetrics = await controlMetrics(carryover);

    const options = await carryover.locator("option").allTextContents();
    expect(options, "en-GB carry-over copy changed — re-measure the id-ID sibling").toContain(
      LONGEST_CARRYOVER_OPTION["en-GB"],
    );

    const available = carryoverMetrics.textWidth - SELECT_ARROW;
    for (const [locale, longest] of Object.entries(LONGEST_CARRYOVER_OPTION)) {
      const rendered = await page.evaluate(
        ({ font, text }) => {
          const ctx = document.createElement("canvas").getContext("2d")!;
          ctx.font = font;
          return ctx.measureText(text).width;
        },
        { font: carryoverMetrics.font, text: longest },
      );
      expect(
        rendered,
        `longest ${locale} carry-over option must fit the select`,
      ).toBeLessThanOrEqual(available);
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

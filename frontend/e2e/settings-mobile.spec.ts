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
// Since #563 the scalar preferences are rows of two group tables and the Data
// flows are panels of a third card, rather than thirteen separate cards. A
// control's reference box is therefore its `[data-slot=settings-row]` or
// `[data-slot=settings-panel]` — the padded box is now the row/panel, not the
// CardContent that holds four of them.
//
// Read-only: it measures the rendered settings surface and writes nothing, so
// there is no seed or cleanup.
//
// At <768px the shell collapses the sidebar (ADR-0025), so navigate by URL.

const FLOOR = 44;

// Geometry of one control relative to the content box of the settings row it
// sits in. Measured against the row rather than a pixel constant so these
// assertions survive a change to the shell, the card, or the row padding —
// since #563 the row is the padded box (one card now holds four of them).
async function controlMetrics(control: Locator) {
  return control.evaluate((el) => {
    const content = el.closest("[data-slot=settings-row]");
    if (!content) throw new Error("control is not inside a settings row");
    const cardStyle = getComputedStyle(content);
    const cardBox = content.getBoundingClientRect();
    const box = el.getBoundingClientRect();
    return {
      width: box.width,
      right: box.right,
      rowInnerWidth:
        cardBox.width - parseFloat(cardStyle.paddingLeft) - parseFloat(cardStyle.paddingRight),
      rowInnerRight: cardBox.right - parseFloat(cardStyle.paddingRight),
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
// per-callsite `w-28` / `w-56` / `w-72` left up to 138px of a 326px card dead.
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
    ] as const) {
      const control = page.getByTestId(testId);
      await control.scrollIntoViewIfNeeded();
      const m = await controlMetrics(control);
      expect(m.width, `${name} should fill the row width`).toBeGreaterThanOrEqual(
        m.rowInnerWidth - 1,
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
      expect(m.width, `${name} should take most of the row`).toBeGreaterThan(m.rowInnerWidth / 2);
      expect(m.width, `${name} should leave the Save at natural width`).toBeLessThan(
        m.rowInnerWidth,
      );
      const save = page.locator(`[data-slot=settings-row]:has(${selector})`).getByRole("button");
      const box = (await save.boundingBox())!;
      expect(box.x + box.width, `${name} Save should sit flush right`).toBeGreaterThanOrEqual(
        m.rowInnerRight - 1,
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
      expect(m.width, `${name} should stay narrow`).toBeLessThan(m.rowInnerWidth / 2);
      const save = page.locator(`[data-slot=settings-row]:has(${selector})`).getByRole("button");
      const box = (await save.boundingBox())!;
      expect(box.x + box.width, `${name} Save should sit flush right`).toBeGreaterThanOrEqual(
        m.rowInnerRight - 1,
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

// The settings-table shape (#563). The two tests above are about one control at
// a time; this one is about the column. Grouping the scalar preferences into
// two cards only pays off if the rows line up — the reason the control column
// is fixed-width and left-aligned inside itself, rather than right-aligned, is
// that right-aligning lines the Saves up but leaves the control left edges
// ragged (a 96px currency box against a full-width select). That is invisible
// in a unit test (jsdom has no layout) and is exactly what regresses when
// someone reaches for a per-row width again.
// covers: INV-PRESENTATION-08
test(
  "settings scalar preferences line up as two-column rows at 1280px",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 1200 });
    await page.goto("/settings");

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    const rows = page.locator("[data-slot=settings-row]");
    // Profile (name / language / theme) + Household (name / currency /
    // multi-currency / inflation).
    await expect(rows).toHaveCount(7);

    const geometry = await rows.evaluateAll((els) =>
      els.map((el) => {
        const [nameCell, controlCell] = Array.from(el.children) as HTMLElement[];
        const save = el.querySelector("button");
        return {
          name: nameCell.textContent?.slice(0, 24) ?? "",
          nameRight: nameCell.getBoundingClientRect().right,
          controlLeft: controlCell.getBoundingClientRect().left,
          controlWidth: controlCell.getBoundingClientRect().width,
          saveRight: save ? save.getBoundingClientRect().right : null,
        };
      }),
    );

    // Two columns, not a stack: the control cell starts after the name cell ends.
    for (const row of geometry) {
      expect(row.controlLeft, `${row.name} row should be two columns`).toBeGreaterThanOrEqual(
        row.nameRight,
      );
    }

    // One shared control column: same left edge and same width on every row,
    // whichever section it belongs to.
    const lefts = new Set(geometry.map((r) => Math.round(r.controlLeft)));
    const widths = new Set(geometry.map((r) => Math.round(r.controlWidth)));
    expect(lefts, "every control shares a left edge").toHaveProperty("size", 1);
    expect(widths, "the control column is one fixed width").toHaveProperty("size", 1);

    // ...and the Saves, which sit at the far end of that column, share a right
    // edge as a consequence.
    const saveRights = new Set(
      geometry.filter((r) => r.saveRight !== null).map((r) => Math.round(r.saveRight!)),
    );
    expect(saveRights.size, "some rows are button-driven").toBeGreaterThan(0);
    expect(saveRights, "every Save shares a right edge").toHaveProperty("size", 1);
  },
);

// The Data section's three primary actions (#563). Each panel is one flow whose
// point is a single button, and at phone width those buttons read as small chips
// parked in an otherwise empty row — so they fill the panel below 768px and
// return to their natural width above it. Measured rather than class-pinned
// because the failure mode is a wrapper that doesn't stretch, not a missing
// utility.
// covers: INV-PRESENTATION-08
test(
  "settings data actions fill the panel at 390px and stay natural width at 1280px",
  { tag: "@smoke" },
  async ({ page }) => {
    const actions = ["backup-export-button", "restore-choose-button", "erase-open-button"] as const;

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/settings");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    for (const testId of actions) {
      const button = page.getByTestId(testId);
      await button.scrollIntoViewIfNeeded();
      const m = await button.evaluate((el) => {
        const panel = el.closest("[data-slot=settings-panel]");
        if (!panel) throw new Error("action is not inside a settings panel");
        const style = getComputedStyle(panel);
        const box = panel.getBoundingClientRect();
        return {
          width: el.getBoundingClientRect().width,
          panelInnerWidth:
            box.width - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
        };
      });
      expect(m.width, `${testId} should fill its panel on a phone`).toBeGreaterThanOrEqual(
        m.panelInnerWidth - 1,
      );
    }

    await page.setViewportSize({ width: 1280, height: 1200 });
    await expect(page.getByTestId("erase-open-button")).toBeVisible();

    for (const testId of actions) {
      const button = page.getByTestId(testId);
      await button.scrollIntoViewIfNeeded();
      const m = await button.evaluate((el) => ({
        width: el.getBoundingClientRect().width,
        panelWidth: el.closest("[data-slot=settings-panel]")!.getBoundingClientRect().width,
      }));
      expect(m.width, `${testId} should stay natural width on desktop`).toBeLessThan(
        m.panelWidth / 2,
      );
    }
  },
);

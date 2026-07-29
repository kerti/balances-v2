import { test, expect } from "@playwright/test";

// Create/edit dialog form bodies at phone width (#541, ADR-0050 /
// INV-PRESENTATION-08). The #428 B-series diverged the *read* surfaces; the
// dialog bodies were deliberately held out and stayed desktop-oriented forms.
//
// A 390px eyeball pass scoped what actually breached the doctrine's bar, and
// most of the suspected breaks did not: the forms fit the viewport, and the
// two-column rows render their dates in full at ~157px, so those stay
// single-layout ("merely cramped" is explicitly not a trigger). What did breach
// was control *size* — every `<select>` sat at 36px because there was no
// `ui/select.tsx` to floor, and the ownership radios had no hit row at all —
// plus one `grid-cols-3` row that truncated the year off its date fields.
//
// The time-deposit create dialog is the representative body: it is the richest
// one, mounting every control kind these forms use (text input, date pair,
// primitive select, the shared OwnershipField radios, and a two-column row).
// If the primitive floor regresses, the select and input assertions fail
// together — that is the signal.
//
// Read-only: it opens the dialog, measures, and dismisses without submitting,
// so there is nothing to seed or clean up.
//
// At <768px the shell collapses the sidebar (ADR-0025), so navigate by URL.

const FLOOR = 44;

// covers: INV-PRESENTATION-08
test(
  "the time-deposit create dialog holds the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/investments/time-deposits");

    await page.getByRole("button", { name: "New time deposit" }).first().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();

    // No horizontal page scroll while the dialog is open — the form body fits
    // the phone width rather than pushing the page sideways.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // The dialog itself must not scroll sideways either — a two-column row that
    // overflows would hide half of a field rather than the page edge.
    const dialogOverflow = await dialog.evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(dialogOverflow).toBeLessThanOrEqual(1);

    // One representative control per kind, all sourced from the primitives.
    const samples: Record<string, ReturnType<typeof page.locator>> = {
      // Input (primitive floor).
      "display name input": dialog.getByLabel("Display name"),
      // Date input — the pair that shares a two-column row.
      "placement date input": dialog.getByLabel("Placement date"),
      "maturity date input": dialog.getByLabel("Maturity date"),
      // Select (the primitive added by #541 — previously 36px everywhere).
      "rollover policy select": dialog.getByLabel("At maturity"),
      "risk profile select": dialog.getByLabel("Risk profile"),
      // Submit / cancel, in the footer.
      "create button": dialog.getByRole("button", { name: "Create" }),
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

    // The ownership radios: the ~13px dot cannot grow, so the label row is the
    // hit area and that is what has to clear the floor.
    for (const option of ["Joint", "Sole owner"]) {
      const row = dialog.locator("label").filter({ hasText: option }).first();
      await row.scrollIntoViewIfNeeded();
      const box = await row.boundingBox();
      expect(box, `${option} row should render`).not.toBeNull();
      expect(box!.height, `${option} row should clear the floor`).toBeGreaterThanOrEqual(FLOOR);
    }
  },
);

// The floor is a mobile rule. Raising these controls app-wide could silently
// pad the desktop forms, which is the regression this pins — same guard
// settings-mobile.spec.ts puts on the Settings surface.
// covers: INV-PRESENTATION-08
test(
  "the time-deposit create dialog keeps its desktop density at 1280px",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/investments/time-deposits");

    await page.getByRole("button", { name: "New time deposit" }).first().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();

    // Natural heights from 768px up: Input/Select are h-8 (32px), the footer
    // buttons h-8. Anything at/above the mobile floor here means a `max-md:`
    // guard was dropped and the floor leaked onto the desktop layout.
    const dense: Record<string, ReturnType<typeof page.locator>> = {
      "display name input": dialog.getByLabel("Display name"),
      "rollover policy select": dialog.getByLabel("At maturity"),
      "create button": dialog.getByRole("button", { name: "Create" }),
    };

    for (const [name, locator] of Object.entries(dense)) {
      const target = locator.first();
      await target.scrollIntoViewIfNeeded();
      const box = await target.boundingBox();
      expect(box, `${name} should render`).not.toBeNull();
      expect(box!.height, `${name} should stay dense on desktop`).toBeLessThan(FLOOR);
    }

    // The three-column term/date row is a desktop layout: it stacks only below
    // 768px, so here the two date fields must sit side by side on one line.
    const start = await dialog.getByLabel("Placement date").first().boundingBox();
    const end = await dialog.getByLabel("Maturity date").first().boundingBox();
    expect(start).not.toBeNull();
    expect(end).not.toBeNull();
    expect(start!.y).toBeCloseTo(end!.y, 0);
  },
);

import { test, expect } from "@playwright/test";

// Income mobile divergence (#508, ADR-0050 B2b "wide table → stacked cards").
// Unlike the dashboard (pure CSS reflow), Income splits at the renderer:
// useIsMobile (768px) mounts IncomeCard on phones and the wide table on desktop.
// This @smoke asserts the correct renderer mounts at each width and the
// ADR-0050 a11y floor holds on the card — the amount (the value the household
// member came for) reads with no horizontal page scroll and the ⋮ action clears
// 44px. Deep per-field assertions stay in the nightly suite; the desktop
// write-flow lives in income.spec.ts.
//
// Seeds one income entry, exercises both widths, and self-cleans afterward.
//
// covers: INV-PRESENTATION-08
test(
  "income mounts the card renderer and holds the mobile a11y floor at 390px",
  {
    tag: "@smoke",
  },
  async ({ page }) => {
    const desc = `E2E income mobile ${Date.now()}`;

    // --- Seed one income entry (desktop width) ---
    await page.goto("/income");
    await page.getByRole("button", { name: "New income" }).first().click();
    const createDialog = page.getByRole("dialog");
    await createDialog.getByLabel("Amount").fill("15000000");
    await createDialog.getByLabel("Category").selectOption("salary");
    await createDialog.getByLabel("Description (optional)").fill(desc);
    await createDialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByTestId("income-table")).toBeVisible();
    await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

    // --- Phone width: the renderer flips from table to cards ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await expect(page.getByTestId("income-card-list")).toBeVisible();
    await expect(page.getByTestId("income-table")).toHaveCount(0);

    // Primary value reachable: the amount reads on the card, within the viewport.
    const amount = page.getByTestId("income-amount").first();
    await expect(amount).toBeVisible();
    await expect(amount).toBeInViewport();

    // No horizontal page scroll — the stacked card fits the phone width.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // Tap-target floor: the card's ⋮ action clears 44px.
    const actions = page.getByRole("button", { name: "Income actions" }).first();
    const box = await actions.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);

    // --- Cleanup: back to desktop, delete the seeded entry ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.reload();
    const row = page.getByRole("row", { name: new RegExp(desc) });
    await row.getByRole("button", { name: "Income actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(desc)).toHaveCount(0);
  },
);

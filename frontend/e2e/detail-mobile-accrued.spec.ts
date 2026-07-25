import { test, expect } from "@playwright/test";

// Detail-page mobile divergence for the accrued snapshot shape (#537, ADR-0051
// Phase B / ADR-0050 "wide table → stacked cards"). Sibling to the qty×price
// @smoke (#536): the two accrued pages (Bond, TimeDeposit) share one
// `AccruedInterestSnapshotRow`/`Card` pair off the generic shell, so the flip is
// proven once here on Bond, the representative subtype. useIsMobile (768px)
// mounts the snapshot card list on phones and the wide table on desktop; this
// asserts the flip and the ADR-0050 a11y floor — the total value reads with no
// horizontal page scroll and the row ⋮ action clears 44px. Deep per-shape
// assertions live nightly.
//
// Seeds a bond + one accrued snapshot, exercises both widths, self-cleans.
//
// covers: INV-PRESENTATION-08
test(
  "accrued detail page mounts the snapshot card renderer and holds the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E accrued mobile ${Date.now()}`;

    // --- Seed a bond + one accrued snapshot (desktop width) ---
    await page.goto("/investments/bonds");
    await page.getByRole("button", { name: "New bond" }).first().click();
    const createDialog = page.getByRole("dialog");
    await createDialog.getByLabel("Display name").fill(name);
    await createDialog.getByLabel("Issuer").fill("E2E Treasury");
    await createDialog.getByLabel("Face value").fill("1000000");
    await createDialog.getByLabel("Coupon rate (% per year)").fill("6.5");
    await createDialog.getByLabel("Maturity date").fill("2030-01-01");
    await createDialog.getByLabel("Placement date").fill("2024-01-01");
    await createDialog.getByLabel("Risk profile").selectOption("medium");
    await createDialog.getByRole("button", { name: "Create" }).click();

    await page
      .getByRole("row", { name: new RegExp(name) })
      .getByText(name)
      .click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

    await page.getByRole("button", { name: "New" }).click();
    const snapDialog = page.getByRole("dialog");
    await snapDialog.getByLabel("Total value (IDR)").fill("1010000");
    await snapDialog.getByLabel("Accrued (IDR)").fill("10000");
    await snapDialog.getByRole("button", { name: "Save snapshot" }).click();

    // Desktop: the snapshot renders in the wide table.
    await expect(page.getByTestId("tour-snapshots-table")).toBeVisible();

    // --- Phone width: the renderer flips from table to cards ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await expect(page.getByTestId("tour-snapshots-cards")).toBeVisible();
    await expect(page.getByTestId("tour-snapshots-table")).toHaveCount(0);

    // Primary value reachable: the total value reads on the card once scrolled
    // to (the bond page's headline/details/chart push the snapshot below the fold
    // at phone height — reachable by vertical scroll is the a11y bar, not above
    // the fold).
    const amount = page.getByTestId("snapshot-amount").first();
    await expect(amount).toBeVisible();
    await amount.scrollIntoViewIfNeeded();
    await expect(amount).toBeInViewport();

    // No horizontal page scroll — the stacked card fits the phone width.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // Tap-target floor: the card's ⋮ action clears 44px.
    const actions = page.getByRole("button", { name: "Snapshot actions" }).first();
    const box = await actions.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);

    // --- Cleanup: back to desktop, delete the snapshot then the position ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.reload();
    await page.getByRole("button", { name: "Snapshot actions" }).first().click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText("No snapshots yet.")).toBeVisible();

    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(name)).toHaveCount(0);
  },
);

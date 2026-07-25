import { test, expect } from "@playwright/test";

// Detail-page mobile divergence (#535, ADR-0051 Phase B / ADR-0050 "wide table →
// stacked cards"). The ten detail pages share one `PositionDetailScreen` shell,
// so the snapshot history reflow lands once in `HistorySection`: useIsMobile
// (768px) mounts the snapshot card list on phones and the wide table on desktop.
// This @smoke asserts the flip on the amount-only shape (BankAccount) and the
// ADR-0050 a11y floor on the card — the amount reads with no horizontal page
// scroll and the row ⋮ action clears 44px. The other snapshot shapes ride the
// same scaffold in #536/#537/#538; deep per-shape assertions live nightly.
//
// Seeds a bank account + one snapshot, exercises both widths, self-cleans.
//
// covers: INV-PRESENTATION-08
test(
  "detail page mounts the snapshot card renderer and holds the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    const account = `E2E detail mobile ${Date.now()}`;

    // --- Seed a bank account + one snapshot (desktop width) ---
    await page.goto("/assets/bank-accounts");
    await page.getByRole("button", { name: "New bank account" }).first().click();
    const acctDialog = page.getByRole("dialog");
    await acctDialog.getByLabel("Display name").fill(account);
    await acctDialog.getByLabel("Bank name").fill("E2E Bank");
    await acctDialog.getByLabel("Account number").fill("1234567890");
    await acctDialog.getByRole("button", { name: "Create" }).click();

    await page
      .getByRole("row", { name: new RegExp(account) })
      .getByText(account)
      .click();
    await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();

    await page.getByRole("button", { name: "New" }).click();
    const snapDialog = page.getByRole("dialog");
    await snapDialog.getByLabel("Amount (IDR)").fill("12500000");
    await snapDialog.getByRole("button", { name: "Save snapshot" }).click();

    // Desktop: the snapshot renders in the wide table.
    await expect(page.getByTestId("tour-snapshots-table")).toBeVisible();

    // --- Phone width: the renderer flips from table to cards ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await expect(page.getByTestId("tour-snapshots-cards")).toBeVisible();
    await expect(page.getByTestId("tour-snapshots-table")).toHaveCount(0);

    // Primary value reachable: the amount reads on the card, within the viewport.
    const amount = page.getByTestId("snapshot-amount").first();
    await expect(amount).toBeVisible();
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

    // #542: the section's *secondary* controls clear 44px too, not only the
    // promoted primary action. The Export trigger stands in for the header /
    // actions-row secondary sweep (Import / carryover / Help / Edit / Terminate /
    // Delete), all raised from `size-sm` to `min-h-11 md:min-h-0` on phones.
    const exportBox = await page.getByTestId("bank-account-export").boundingBox();
    expect(exportBox).not.toBeNull();
    expect(exportBox!.height).toBeGreaterThanOrEqual(44);

    // --- Cleanup: back to desktop, delete the snapshot then the account ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.reload();
    await page.getByRole("row", { name: /12,500,000|12500000/ }).first();
    await page.getByRole("button", { name: "Snapshot actions" }).first().click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText("No snapshots yet.")).toBeVisible();

    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(account)).toHaveCount(0);
  },
);

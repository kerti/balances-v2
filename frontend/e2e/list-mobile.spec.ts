import { test, expect } from "@playwright/test";

// Position-list mobile a11y floor (#510, ADR-0050). The ten descriptor-driven
// list screens (ADR-0043) already diverge table→cards via PositionListScreen →
// PositionListCards / PositionListTable (useIsMobile, 768px; INV-PRESENTATION-05
// verifies that renderer *contract*). This @smoke asserts the a11y *floor*
// (INV-PRESENTATION-08) the cards path never carried: on the mobile card the row
// ⋮ action clears the 44px tap-target floor and the layout holds with no
// horizontal page scroll. Bank accounts stands in for the shared renderer — all
// ten position types render the same `PositionListCards` + `RowActionsMenu`.
//
// covers: INV-PRESENTATION-08
test(
  "position-list cards hold the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    const account = `E2E list mobile account ${Date.now()}`;

    // --- Seed one bank account (a card needs no snapshot to render) ---
    await page.goto("/assets/bank-accounts");
    await page.getByRole("button", { name: "New bank account" }).first().click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Display name").fill(account);
    await dialog.getByLabel("Bank name").fill("E2E Bank");
    await dialog.getByLabel("Account number").fill("1234567890");
    await dialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(account) })).toBeVisible();

    // --- Phone width: the list renders as cards (PositionListCards) ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    const card = page.getByTestId("bank-account-card").filter({ hasText: account });
    await expect(card).toBeVisible();

    // No horizontal page scroll — the card layout stays within the viewport.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // Tap-target floor: the row ⋮ action sizes to ≥44px on the card.
    const actions = card.getByRole("button", { name: "Bank account actions" });
    await expect(actions).toBeVisible();
    const box = await actions.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.width).toBeGreaterThanOrEqual(44);

    // Header action buttons are promoted to primary actions on mobile (ADR-0050):
    // they size up to the 44px floor and split the row (Import + create → half
    // each here). Assert the floor height and that the pair fills the row.
    const createBtn = page.getByRole("button", { name: "New bank account" });
    const importBtn = page.getByRole("button", { name: "Import", exact: true });
    const createBox = await createBtn.boundingBox();
    const importBox = await importBtn.boundingBox();
    expect(createBox).not.toBeNull();
    expect(importBox).not.toBeNull();
    expect(createBox!.height).toBeGreaterThanOrEqual(44);
    expect(importBox!.height).toBeGreaterThanOrEqual(44);
    // Two buttons split the width — each takes roughly (but well over a third of)
    // the 390px row, which the old content-width dense buttons never did.
    expect(createBox!.width).toBeGreaterThan(130);
    expect(importBox!.width).toBeGreaterThan(130);

    // --- Cleanup: desktop width, delete via the table row ⋮ ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.reload();
    const row = page.getByRole("row", { name: new RegExp(account) });
    await row.getByRole("button", { name: "Bank account actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(account)).toHaveCount(0);
  },
);

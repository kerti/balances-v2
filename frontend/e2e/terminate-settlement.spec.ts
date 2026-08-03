import { test, expect } from "@playwright/test";

// Capture-at-source through the real UI + backend (ADR-0052 §6, #587): closing
// an Investment books the Sell that says where its value went, on the same
// request as the status flip. Before this, a user who flipped the status and
// never recorded the Sell had the whole position booked as a total loss.
//
// The atomicity of the two writes is a repo-level guarantee
// (repo/investment_settlement_test.go); what this proves end to end is that one
// dialog submission produces both the flip and the ledger row. Self-cleaning per
// ADR-0024.
test(
  "closing a stock captures the sale on the same request",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E settle ${Date.now()}`;
    const statusBadge = page.getByTestId("status-badge");

    await page.goto("/investments/stocks");

    // --- Create the stock position ---
    await page.getByRole("button", { name: "New stock" }).first().click();
    const createDialog = page.getByRole("dialog");
    await expect(createDialog.getByText("New stock position")).toBeVisible();
    await createDialog.getByLabel("Display name").fill(name);
    await createDialog.getByLabel("Ticker").fill("E2ES");
    await createDialog.getByLabel("Exchange").fill("IDX");
    await createDialog.getByLabel("Risk profile").selectOption("medium");
    await createDialog.getByRole("button", { name: "Create" }).click();

    await page
      .getByRole("row", { name: new RegExp(name) })
      .getByText(name)
      .click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

    // --- Buy 100 @ 9,500, so the ledger carries a held quantity to close out ---
    await page.getByRole("button", { name: "Buy" }).click();
    const buyDialog = page.getByRole("dialog");
    await buyDialog.getByLabel(/Quantity/).fill("100");
    await buyDialog.getByLabel(/Price per unit/).fill("9500");
    await buyDialog.getByRole("button", { name: "Record buy" }).click();
    await expect(page.getByRole("row", { name: /Buy/ })).toBeVisible();

    // --- Close the position; the settlement block appears with the terminal
    //     status and is pre-filled from the ledger's held quantity ---
    await page.getByRole("button", { name: "Close", exact: true }).click();
    const closeDialog = page.getByRole("dialog");
    await closeDialog.getByLabel("Status").selectOption("sold");

    const settlement = closeDialog.getByTestId("terminate-settlement");
    await expect(settlement).toBeVisible();
    await expect(closeDialog.getByTestId("settlement-quantity")).toHaveValue("100");
    // Sold above the marked price — the gain the ledger should end up showing.
    await closeDialog.getByTestId("settlement-price").fill("11000");
    await closeDialog.getByRole("button", { name: "Save" }).click();

    // One submission, both effects: the status flipped AND the Sell is on the
    // ledger without a second action.
    await expect(statusBadge).toHaveText("Sold");
    await expect(page.getByRole("row", { name: /Sell/ })).toBeVisible();

    // --- Delete (cleanup — returns to the empty list) ---
    await page.getByRole("button", { name: "Delete" }).click();
    const confirm = page.getByRole("alertdialog");
    await confirm.getByRole("button", { name: "Delete" }).click();

    await expect(page.getByText(name)).toHaveCount(0);
  },
);

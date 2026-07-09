import { test, expect } from "@playwright/test";

// Bulk monthly-entry for the Asset group through the real UI + backend
// (ADR-0046): from Assets home, open "Enter this month", find a freshly-created
// bank account listed with no carried-forward value, type an amount, save all,
// then re-open the same month and confirm the row now warns it will overwrite —
// proving the batch persisted and the carry-forward surfacing flips to the
// same-month upsert state. Self-cleaning: deletes the parent account (taking its
// snapshot with it), leaving the seed's empty bank-account list. See ADR-0024.
// covers: INV-JOURNEYS-05
test(
  "asset bulk monthly-entry: list → save → re-open shows overwrite",
  { tag: "@smoke" },
  async ({ page }) => {
    const account = `E2E bulk account ${Date.now()}`;

    await page.goto("/assets/bank-accounts");

    // --- Create the parent bank account (no snapshots yet) ---
    await page.getByRole("button", { name: "New bank account" }).first().click();
    const acctDialog = page.getByRole("dialog");
    await acctDialog.getByLabel("Display name").fill(account);
    await acctDialog.getByLabel("Bank name").fill("E2E Bank");
    await acctDialog.getByLabel("Account number").fill("1234567890");
    await acctDialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(account) })).toBeVisible();

    // --- Open the bulk monthly-entry view from Assets home ---
    await page.goto("/assets");
    await expect(page.getByTestId("assets-home")).toBeVisible();
    await page.getByTestId("assets-enter-month").click();
    await expect(page.getByText("Enter this month's balances")).toBeVisible();

    // The new account is listed with no history and nothing counted as changed.
    const row = page.locator("li").filter({ hasText: account });
    await expect(row.getByText("No previous value")).toBeVisible();
    await expect(page.getByTestId("asset-entry-dirty-count")).toHaveText("0 changed");

    // --- Type a value: the row goes dirty and the change count ticks up ---
    await row.getByRole("textbox").fill("12500000");
    await expect(page.getByTestId("asset-entry-dirty-count")).toHaveText("1 changed");

    // Pin the statement date to the first of the chosen month rather than the
    // browser-local "today" the screen seeds: the backend's future-date guard
    // compares in UTC, so a UTC+ tester running pre-dawn would send a date one
    // calendar day ahead of the server's UTC today and the save would 400. The
    // first-of-month is unambiguously past and in-month in every timezone.
    const month = await page.getByTestId("asset-entry-month").inputValue();
    await page.getByTestId("asset-entry-asof").fill(`${month}-01`);

    // --- Save all: a successful batch redirects back to Assets home ---
    await page.getByTestId("asset-entry-save").click();
    await expect(page).toHaveURL(/\/assets$/);
    await expect(page.getByTestId("assets-home")).toBeVisible();

    // --- Re-open the same month: the write persisted, so the row now carries
    //     forward this month's own value and warns the next save overwrites it. ---
    await page.getByTestId("assets-enter-month").click();
    const savedRow = page.locator("li").filter({ hasText: account });
    await expect(savedRow.getByText("Will overwrite")).toBeVisible();
    await expect(savedRow.getByRole("textbox")).toHaveValue("12500000");

    // --- Cleanup: delete the parent account (returns to the empty list) ---
    await page.goto("/assets/bank-accounts");
    await page
      .getByRole("row", { name: new RegExp(account) })
      .getByText(account)
      .click();
    await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();
    await page.getByRole("button", { name: "Delete" }).click();
    const acctConfirm = page.getByRole("alertdialog");
    await acctConfirm.getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(account)).toHaveCount(0);
  },
);

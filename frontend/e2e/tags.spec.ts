import { test, expect } from "@playwright/test";

// User-defined Tags smoke (ADR-0028, slice 2). Covers the three new surfaces:
// the Settings Tags management card (create + delete), the detail-screen
// DetailTagControl (assign + persist across reload), and the /tags report
// route. Self-cleaning: deletes the bank account and the tag it creates, so the
// seed's empty tag + bank-account lists are restored. A unique tag name per run
// dodges the per-household unique-name constraint if a prior run died dirty.
test("tags: create in settings, assign on a position, report route, cleanup", async ({ page }) => {
  const tagName = `E2E Tag ${Date.now()}`;
  const acctName = `E2E Tagged Acct ${Date.now()}`;

  // --- Create a tag in Settings ---
  await page.goto("/settings");
  await page.getByTestId("new-tag-name").fill(tagName);
  await page.getByTestId("add-tag").click();
  await expect(page.getByTestId("tag-list").getByText(tagName)).toBeVisible();

  // --- Create a bank account to tag ---
  await page.goto("/assets/bank-accounts");
  await page.getByRole("button", { name: "New bank account" }).first().click();
  const createDialog = page.getByRole("dialog");
  await createDialog.getByLabel("Display name").fill(acctName);
  await createDialog.getByLabel("Bank name").fill("E2E Bank");
  await createDialog.getByLabel("Account number").fill("1234567890");
  await createDialog.getByRole("button", { name: "Create" }).click();

  // Open its detail.
  await page
    .getByRole("row", { name: new RegExp(acctName) })
    .getByText(acctName)
    .click();
  await expect(page.getByRole("heading", { level: 1, name: acctName })).toBeVisible();

  // --- Assign the tag via the detail-screen control ---
  const tagSelect = page.getByTestId("tag-select");
  // Wait for the assign PUT to settle before reloading — selectOption fires the
  // mutation but doesn't await it, so a bare reload races the network call.
  const assigned = page.waitForResponse(
    (r) => r.url().includes("/api/tags/assignments") && r.request().method() === "PUT" && r.ok(),
  );
  await tagSelect.selectOption({ label: tagName });
  await assigned;
  // Persisted across a reload (proves the assign call, not just local state).
  await page.reload();
  await expect(page.getByTestId("tag-select").locator("option:checked")).toHaveText(tagName);

  // --- /tags report route renders ---
  await page.goto("/tags");
  await expect(page.getByRole("heading", { level: 1, name: "Tags" })).toBeVisible();

  // --- Cleanup: delete the bank account, then the tag ---
  await page.goto("/assets/bank-accounts");
  await page
    .getByRole("row", { name: new RegExp(acctName) })
    .getByText(acctName)
    .click();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();

  await page.goto("/settings");
  await page.getByTestId("delete-tag").click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: /delete/i })
    .click();
  await expect(page.getByTestId("tag-list").getByText(tagName)).toHaveCount(0);
});

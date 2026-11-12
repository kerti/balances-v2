import { test, expect, type Page } from "@playwright/test";
import { backToListAfterCreate } from "./landing";

// One list pattern (#838): the Ownership, Status and Value columns sit at the
// same x on every position list at 1280px, Name is the only column that flexes
// (ticking "show inactive" adds Status without moving Value), and a long note
// in a snapshot row stays inside its own cell instead of pushing Amount along.
// Bank accounts and stocks stand in for the non-investment and investment
// presets; every list renders the same PositionListTable.
//
// covers: INV-PRESENTATION-24
test("list columns align across lists and do not reflow", { tag: "@smoke" }, async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const account = `E2E cols account ${Date.now()}`;
  const stock = `E2E cols stock ${Date.now()}`;
  const longNote = `E2E long note ${"lorem ipsum dolor sit amet ".repeat(8)}`;

  const x = async (p: Page, testId: string) => {
    const box = await p.getByTestId(testId).boundingBox();
    expect(box).not.toBeNull();
    return Math.round(box!.x);
  };

  // --- Bank account with an unnoted snapshot ---
  await page.goto("/assets/bank-accounts");
  await page.getByRole("button", { name: "New bank account" }).first().click();
  const acctDialog = page.getByRole("dialog");
  await acctDialog.getByLabel("Display name").fill(account);
  await acctDialog.getByLabel("Bank name").fill("E2E Bank");
  await acctDialog.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(/\/assets\/bank-accounts\/[0-9a-f-]{36}$/);
  const snapDialog = page.getByRole("dialog");
  await expect(snapDialog.getByText("Record monthly snapshot")).toBeVisible();
  await snapDialog.getByLabel("Amount (IDR)").fill("12,500,000");
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
  const snapshotCard = page.getByTestId("tour-snapshots");
  await expect(snapshotCard.getByTestId("snapshot-row")).toHaveCount(1);

  // --- A long note stays in its own cell: Amount does not move ---
  const amountBefore = await x(page, "snapshot-amount");
  await snapshotCard.getByRole("button", { name: "Snapshot actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const editDialog = page.getByRole("dialog");
  await editDialog.getByLabel("Note (optional)").fill(longNote);
  await editDialog.getByRole("button", { name: "Save changes" }).click();
  await expect(snapshotCard.getByTestId("snapshot-row")).toContainText("E2E long note");
  expect(await x(page, "snapshot-amount")).toBe(amountBefore);

  // --- Close the account so the list offers "show inactive" ---
  await page.getByTestId("position-menu-trigger").click();
  await page.getByRole("menuitem", { name: "Close…", exact: true }).click();
  const closeDialog = page.getByRole("dialog");
  await closeDialog.getByLabel("Status").selectOption("closed");
  await closeDialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("status-badge")).toHaveText("Closed");

  // --- Stock, closed the same way ---
  await page.goto("/investments/stocks");
  await page.getByRole("button", { name: "New stock" }).first().click();
  const stockDialog = page.getByRole("dialog");
  await stockDialog.getByLabel("Display name").fill(stock);
  await stockDialog.getByLabel("Ticker").fill("E2EC");
  await stockDialog.getByLabel("Exchange").fill("IDX");
  await stockDialog.getByLabel("Risk profile").selectOption("medium");
  await stockDialog.getByRole("button", { name: "Create" }).click();
  await backToListAfterCreate(page);
  // Measured while this stock is still active, so the table renders whatever
  // other specs left behind; Status is hidden until "show inactive" is ticked.
  await expect(page.getByTestId("sort-ownership")).toBeVisible();
  await expect(page.getByTestId("sort-status")).toHaveCount(0);
  const valueHidden = await x(page, "sort-value");
  await page
    .getByRole("row", { name: new RegExp(stock) })
    .getByText(stock)
    .click();
  await page.getByTestId("position-menu-trigger").click();
  await page.getByRole("menuitem", { name: "Close…", exact: true }).click();
  const stockClose = page.getByRole("dialog");
  await stockClose.getByLabel("Status").selectOption("untracked");
  await stockClose.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("status-badge")).toHaveText("No longer tracked");

  // --- Stocks list with Status: tick "show inactive" ---
  await page.goto("/investments/stocks");
  await page.getByTestId("show-inactive").check();
  await expect(page.getByTestId("sort-status")).toBeVisible();
  const stockCols = {
    ownership: await x(page, "sort-ownership"),
    status: await x(page, "sort-status"),
    value: await x(page, "sort-value"),
  };
  // Status appearing widens nothing but Name: Value stays put.
  expect(stockCols.value).toBe(valueHidden);

  // --- Bank list: same x for the same roles ---
  await page.goto("/assets/bank-accounts");
  await page.getByTestId("show-inactive").check();
  await expect(page.getByTestId("sort-status")).toBeVisible();
  expect(await x(page, "sort-ownership")).toBe(stockCols.ownership);
  expect(await x(page, "sort-status")).toBe(stockCols.status);
  expect(await x(page, "sort-value")).toBe(stockCols.value);

  // --- Cleanup: delete both from their detail pages ---
  await page
    .getByRole("row", { name: new RegExp(account) })
    .getByText(account)
    .click();
  await page.getByTestId("position-menu-trigger").click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("main").getByText(account)).toHaveCount(0);

  await page.goto("/investments/stocks");
  await page.getByTestId("show-inactive").check();
  await page
    .getByRole("row", { name: new RegExp(stock) })
    .getByText(stock)
    .click();
  await page.getByTestId("position-menu-trigger").click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("main").getByText(stock)).toHaveCount(0);
});

import { test, expect } from "@playwright/test";
import { pickMonth } from "./monthPicker";

// Amount-only snapshot journey through the real UI + backend (ADR-0022): on a
// bank account detail page, Record snapshot for last month, see that the next
// open defaults to this month with a "Same as last month" helper, replace the
// recorded month with an Undo, then edit and delete it. Assertions key off a
// unique description; the amount renders IDR-formatted and is a poor anchor.
// Self-cleaning: deletes the snapshot and then the parent account, leaving the
// seed's empty bank-account list. See ADR-0024.
// The Record snapshot legs prove INV-JOURNEYS-02 (ADR-0046 amendment, #788); the
// delete leg also proves the no-dialog + Undo journey.
// covers: INV-JOURNEYS-02, INV-JOURNEYS-06
test(
  "bank account snapshot record → replace → edit → delete",
  { tag: "@smoke" },
  async ({ page }) => {
    const account = `E2E snap account ${Date.now()}`;
    const desc = `E2E snapshot ${Date.now()}`;
    const editedDesc = `${desc} edited`;
    const now = new Date();
    const lastMonth = new Date(now.getFullYear(), now.getMonth() - 1, 1);
    const lastYm = `${lastMonth.getFullYear()}-${String(lastMonth.getMonth() + 1).padStart(2, "0")}`;

    await page.goto("/assets/bank-accounts");

    // --- Create the parent bank account ---
    await page.getByRole("button", { name: "New bank account" }).first().click();
    const acctDialog = page.getByRole("dialog");
    await acctDialog.getByLabel("Display name").fill(account);
    await acctDialog.getByLabel("Bank name").fill("E2E Bank");
    // Account number is optional (#843): left blank here.
    await acctDialog.getByRole("button", { name: "Create" }).click();

    // --- Creating lands on the new account with Record snapshot already open
    //     (#843); record a snapshot for last month (it opens on this month) ---
    // The open modal hides the page from the a11y tree, so the landing is
    // asserted by URL here; the heading is checked once the dialog closes.
    await expect(page).toHaveURL(/\/assets\/bank-accounts\/[0-9a-f-]{36}$/);
    const createDialog = page.getByRole("dialog");
    await expect(createDialog.getByText("Record monthly snapshot")).toBeVisible();
    await expect(createDialog.getByTestId("snapshot-replace-notice")).toHaveCount(0);
    await pickMonth(page, createDialog.getByTestId("year_month"), lastYm);
    await createDialog.getByLabel("Amount (IDR)").fill("12,500,000");
    await createDialog.getByLabel("Note (optional)").fill(desc);
    await createDialog.getByRole("button", { name: "Save snapshot" }).click();

    const row = page.getByRole("row", { name: new RegExp(desc) });
    await expect(row).toBeVisible();
    await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();

    // --- Record snapshot again: it defaults to the month after the latest (this
    //     month, empty), starts with an empty amount, and "Same as last month"
    //     fills it while the statement date stays blank. Cancel without writing. ---
    await page.getByRole("button", { name: "Record snapshot" }).click();
    const nextDialog = page.getByRole("dialog");
    await expect(nextDialog.getByTestId("snapshot-replace-notice")).toHaveCount(0);
    await expect(nextDialog.getByLabel("Amount (IDR)")).toHaveValue("");
    await expect(nextDialog.getByTestId("snapshot-last-recorded")).toBeVisible();
    await nextDialog.getByTestId("snapshot-same-as-last").click();
    await expect(nextDialog.getByLabel("Amount (IDR)")).toHaveValue("12,500,000");
    await expect(nextDialog.getByLabel("Statement date (optional)")).toHaveValue("");
    await nextDialog.getByRole("button", { name: "Cancel" }).click();

    // --- Pick the already-recorded month: the dialog switches to replacing it,
    //     with a notice before saving and the value prefilled. Saving replaces
    //     (one row, not two) and the toast offers Undo. ---
    await page.getByRole("button", { name: "Record snapshot" }).click();
    const replaceDialog = page.getByRole("dialog");
    await pickMonth(page, replaceDialog.getByTestId("year_month"), lastYm);
    await expect(replaceDialog.getByTestId("snapshot-replace-notice")).toBeVisible();
    await expect(replaceDialog.getByLabel("Amount (IDR)")).toHaveValue("12,500,000");
    await replaceDialog.getByLabel("Amount (IDR)").fill("13,000,000");
    await replaceDialog.getByRole("button", { name: "Save snapshot" }).click();

    await expect(page.getByRole("row", { name: new RegExp(desc) })).toHaveCount(1);
    await expect(row).toContainText("13,000,000");
    await page.getByTestId("toast-replaced").getByRole("button", { name: "Undo" }).click();
    await expect(row).toContainText("12,500,000");

    // --- Edit the snapshot (change the description) ---
    await row.getByRole("button", { name: "Snapshot actions" }).click();
    await page.getByRole("menuitem", { name: "Edit" }).click();
    const editDialog = page.getByRole("dialog");
    await expect(editDialog.getByText("Edit snapshot")).toBeVisible();
    await editDialog.getByLabel("Note (optional)").fill(editedDesc);
    await editDialog.getByRole("button", { name: "Save changes" }).click();

    const editedRow = page.getByRole("row", { name: new RegExp(editedDesc) });
    await expect(editedRow).toBeVisible();

    // --- Delete the snapshot (table returns to its empty state) ---
    await editedRow.getByRole("button", { name: "Snapshot actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();

    // No confirm dialog (ADR-0058 §3): the row goes at once and the toast offers
    // Undo, which brings the very row back.
    await expect(page.getByRole("alertdialog")).toHaveCount(0);
    await expect(page.getByRole("main").getByText(editedDesc)).toHaveCount(0);
    await expect(page.getByText("No snapshots yet.")).toBeVisible();
    await page.getByTestId("toast-deleted").getByRole("button", { name: "Undo" }).click();
    await expect(editedRow).toBeVisible();

    // --- Delete it again for good (cleanup) ---
    await editedRow.getByRole("button", { name: "Snapshot actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await expect(page.getByRole("main").getByText(editedDesc)).toHaveCount(0);
    await expect(page.getByText("No snapshots yet.")).toBeVisible();

    // --- Delete the parent account (cleanup — returns to the empty list) ---
    await page.getByTestId("position-menu-trigger").click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    const acctConfirm = page.getByRole("alertdialog");
    await acctConfirm.getByRole("button", { name: "Delete" }).click();

    await expect(page.getByRole("main").getByText(account)).toHaveCount(0);
  },
);

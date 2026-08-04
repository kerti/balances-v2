import { test, expect } from "@playwright/test";

// The exit side of a Tracking Change through the real UI + backend (ADR-0053
// §5, #595): a Position can be terminated as `untracked` — it left the
// household's books because their coverage changed, not because it was sold,
// paid off or lost.
//
// Driven on an Investment on purpose. Every other group's terminate dialog is
// unconstrained, but an Investment's terminal statuses are narrowed to the ones
// its transaction matrix can settle (ADR-0052 §6), and terminating it demands
// the settling Sell/Maturity. `untracked` is the one status exempt from both —
// nothing was sold, so there are no proceeds to capture. If the narrowing or the
// backend's own carve-out regresses, this status becomes unreachable for the
// whole group and the household's only truthful option is to book a departing
// portfolio as a total loss (the #576 falsehood).
//
// The Tracking Changes figure the termination feeds is derived server-side and
// proved at the engine tier (repo/monthly_reports_engine_trackingchanges_test.go);
// what this proves end to end is that one dialog submission reaches it without a
// settlement. Self-cleaning per ADR-0024.
// covers: INV-LIFECYCLE-09
test(
  "an investment terminates as untracked with no settlement, and reactivates",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E untracked ${Date.now()}`;
    const statusBadge = page.getByTestId("status-badge");
    const snapshotCard = page.getByTestId("tour-snapshots");

    await page.goto("/investments/stocks");

    // --- Create the stock position ---
    await page.getByRole("button", { name: "New stock" }).first().click();
    const createDialog = page.getByRole("dialog");
    await expect(createDialog.getByText("New stock position")).toBeVisible();
    await createDialog.getByLabel("Display name").fill(name);
    await createDialog.getByLabel("Ticker").fill("E2EU");
    await createDialog.getByLabel("Exchange").fill("IDX");
    await createDialog.getByLabel("Risk profile").selectOption("medium");
    await createDialog.getByRole("button", { name: "Create" }).click();

    await page
      .getByRole("row", { name: new RegExp(name) })
      .getByText(name)
      .click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

    // --- Mark it at 100 @ 9,500, so there is a real value to leave the books ---
    await page.getByRole("button", { name: "New" }).click();
    const snapDialog = page.getByRole("dialog");
    await expect(snapDialog.getByText("Record monthly snapshot")).toBeVisible();
    await snapDialog.getByLabel("Quantity", { exact: true }).fill("100");
    await snapDialog.getByLabel("Price per unit (IDR)").fill("9500");
    await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
    await expect(snapshotCard.getByTestId("snapshot-row")).toHaveCount(1);
    await expect(snapshotCard.getByTestId("snapshot-amount")).toContainText("950");

    // --- Terminate as untracked ---
    await page.getByRole("button", { name: "Close", exact: true }).click();
    const closeDialog = page.getByRole("dialog");
    await expect(closeDialog.getByText("Close position")).toBeVisible();

    // The status is offered at all — the subtype narrowing lets it through.
    await closeDialog.getByLabel("Status").selectOption("untracked");

    // The exemption: no settlement block for a status that settles nothing, and
    // the copy that distinguishes it from the cash-settled statuses above it.
    await expect(closeDialog.getByTestId("terminate-untracked-hint")).toBeVisible();
    await expect(closeDialog.getByTestId("terminate-settlement")).toHaveCount(0);

    await closeDialog.getByRole("button", { name: "Save" }).click();

    // Accepted without a Sell: the status flips, the 0-value close snapshot
    // displaces the marked one (INV-LIFECYCLE-03 applies unchanged), and the
    // ledger stays empty — nothing was sold.
    await expect(statusBadge).toHaveText("No longer tracked");
    await expect(snapshotCard.getByTestId("snapshot-row")).toHaveCount(1);
    await expect(snapshotCard.getByTestId("snapshot-amount")).not.toContainText("950");
    await expect(page.getByRole("row", { name: /Sell/ })).toHaveCount(0);

    // --- Reactivate: the displaced snapshot is handed back the way it is for
    //     every other terminal status — the close row goes, the 9,500 mark
    //     returns ---
    await page.getByRole("button", { name: "Status", exact: true }).click();
    const reopenDialog = page.getByRole("dialog");
    await reopenDialog.getByLabel("Status").selectOption("active");
    await reopenDialog.getByRole("button", { name: "Save" }).click();

    await expect(statusBadge).toHaveText("Active");
    await expect(snapshotCard.getByTestId("snapshot-row")).toHaveCount(1);
    await expect(snapshotCard.getByTestId("snapshot-amount")).toContainText("950");

    // --- Delete (cleanup — returns to the empty list) ---
    await page.getByRole("button", { name: "Delete" }).click();
    const confirm = page.getByRole("alertdialog");
    await confirm.getByRole("button", { name: "Delete" }).click();

    await expect(page.getByText(name)).toHaveCount(0);
  },
);

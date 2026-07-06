import { test, expect } from "@playwright/test";

// Settings → Data → Backup export (ADR-0036, issue #174). Seeded Alice is
// auto-authenticated (global-setup). The button fetches the gzip stream and
// triggers a browser download; we assert the suggested filename matches the
// .json.gz contract and that full fidelity is the default selection.
//
// covers: INV-BACKUP-01, INV-BACKUP-04
test("settings backup export downloads a .json.gz file", { tag: "@smoke" }, async ({ page }) => {
  await page.goto("/settings");

  const exportBtn = page.getByTestId("backup-export-button");
  await expect(exportBtn).toBeVisible();
  // Full fidelity is the safe default (lossless).
  await expect(page.getByTestId("backup-fidelity-full")).toBeChecked();

  const downloadPromise = page.waitForEvent("download");
  await exportBtn.click();
  const download = await downloadPromise;

  expect(download.suggestedFilename()).toMatch(/^household-backup-\d{4}-\d{2}-\d{2}\.json\.gz$/);
});

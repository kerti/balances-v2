import { test, expect } from "@playwright/test";

// Dashboard → "Download PDF" (#187, ADR-0044). Seeded Alice is
// auto-authenticated (global-setup). Client-side generation via
// @react-pdf/renderer triggers a browser download; we assert only the
// suggested filename — no PDF byte/content diffing (see ADR-0044's
// "where automated coverage stops").
test("dashboard PDF export downloads a PDF file", { tag: "@smoke" }, async ({ page }) => {
  await page.goto("/");

  // The button is behind a lazy import() (ReportPdfButton, ADR-0044) whose
  // chunk (~1.4MB — @react-pdf/renderer's font/layout engine) is far larger
  // than any other lazy chunk in this app; the default 5s locator timeout can
  // be too tight for that one-time fetch+eval on a cold-cache CI run.
  const btn = page.getByTestId("download-pdf-button");
  await expect(btn).toBeVisible({ timeout: 15_000 });

  const downloadPromise = page.waitForEvent("download");
  await btn.click();
  const download = await downloadPromise;

  expect(download.suggestedFilename()).toMatch(/^Balances_\d{4}-\d{2}\.pdf$/);
});

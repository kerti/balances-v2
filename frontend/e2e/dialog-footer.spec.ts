import { test, expect, type Page } from "@playwright/test";

// A form dialog's footer stays pinned inside the dialog after an inline
// validation error grows the body (#938): submit a create dialog empty at a
// 1366x757 laptop viewport and measure Cancel and Create against the dialog's
// visible box. Only the body scrolls.
//
// covers: INV-PRESENTATION-33
test.use({ viewport: { width: 1366, height: 757 } });

async function expectFooterInsideDialog(page: Page) {
  const dialog = page.getByRole("dialog");
  const box = await dialog.boundingBox();
  expect(box, "dialog box").not.toBeNull();
  for (const name of ["Cancel", "Create"]) {
    const button = dialog.getByRole("button", { name, exact: true });
    await expect(button).toBeVisible();
    const b = await button.boundingBox();
    expect(b, `${name} box`).not.toBeNull();
    expect(b!.y).toBeGreaterThanOrEqual(box!.y);
    expect(b!.y + b!.height).toBeLessThanOrEqual(box!.y + box!.height + 0.5);
  }
}

for (const [label, path, trigger] of [
  ["bank account", "/assets/bank-accounts", "New bank account"],
  ["liability", "/liabilities/personal", "New liability"],
] as const) {
  test(
    `${label} dialog keeps Cancel and Create inside it after a validation error`,
    { tag: "@smoke" },
    async ({ page }) => {
      await page.goto(path);
      await page.getByRole("button", { name: trigger }).first().click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await expectFooterInsideDialog(page);

      await page.getByRole("dialog").getByRole("button", { name: "Create", exact: true }).click();
      await expect(page.getByRole("dialog").locator('[aria-invalid="true"]').first()).toBeVisible();
      await expectFooterInsideDialog(page);
    },
  );
}

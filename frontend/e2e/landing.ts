import { expect, type Page } from "@playwright/test";

// Creating a position lands on its detail page with Record snapshot already open
// (#843). Specs that only need the new row on its list call this right after the
// Create click: it asserts the landing, cancels the dialog (the item keeps no
// snapshot) and steps back to the list the create started from.
export async function backToListAfterCreate(page: Page) {
  const record = page.getByRole("dialog");
  await expect(record.getByText("Record monthly snapshot")).toBeVisible();
  await record.getByRole("button", { name: "Cancel" }).click();
  await expect(record).toHaveCount(0);
  await page.goBack();
}

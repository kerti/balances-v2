import { test, expect, type Page } from "@playwright/test";
import { backToListAfterCreate } from "./landing";

// User-defined Tags (ADR-0028, #839). A Tag is set only in a position's Create
// and Edit dialogs, saved by the same Save as the rest of the position. Covers:
// the /tags page's management card (create + delete), a tag chosen in the Create
// dialog, then changed and cleared in the Edit dialog (each persisted), and the
// report route. The tag and the account are this spec's own (unique names), so
// the spec relies on no other spec's data and is self-cleaning: it deletes the
// bank account and the tag it creates, restoring the seed's empty lists.
async function taggedItemTagId(page: Page, name: string): Promise<string | null | undefined> {
  const res = await page.request.get("/api/tags/items");
  expect(res.ok()).toBe(true);
  const items = (await res.json()) as { name: string; tag_id: string | null }[];
  return items.find((it) => it.name === name)?.tag_id;
}

test("tags: set in Create, change and clear in Edit, report renders, cleanup", async ({ page }) => {
  const tagName = `E2E Tag ${Date.now()}`;
  const acctName = `E2E Tagged Acct ${Date.now()}`;

  // --- Create a tag on the Tags page ---
  await page.goto("/tags");
  await page.getByTestId("new-tag").click();
  const tagDialog = page.getByRole("dialog");
  await tagDialog.getByLabel("Name").fill(tagName);
  await tagDialog.getByRole("button", { name: "Add" }).click();
  await expect(page.getByTestId("tag-list").getByText(tagName)).toBeVisible();

  const tagsRes = await page.request.get("/api/tags");
  const tagId = ((await tagsRes.json()) as { id: string; name: string }[]).find(
    (t) => t.name === tagName,
  )?.id;
  expect(tagId).toBeTruthy();

  // --- Create a bank account with the tag chosen in the Create dialog ---
  await page.goto("/assets/bank-accounts");
  await page.getByRole("button", { name: "New bank account" }).first().click();
  const createDialog = page.getByRole("dialog");
  await createDialog.getByLabel("Display name").fill(acctName);
  await createDialog.getByLabel("Bank name").fill("E2E Bank");
  await createDialog.getByLabel("Account number").fill("1234567890");
  await createDialog.getByTestId("tag-select").selectOption({ label: tagName });
  await createDialog.getByRole("button", { name: "Create" }).click();
  await backToListAfterCreate(page);
  // Written by the create itself, not a second call.
  expect(await taggedItemTagId(page, acctName)).toBe(tagId);

  // Open its detail and its Edit dialog: the Tag is preselected.
  await page
    .getByRole("row", { name: new RegExp(acctName) })
    .getByText(acctName)
    .click();
  await expect(page.getByRole("heading", { level: 1, name: acctName })).toBeVisible();
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  const editDialog = page.getByRole("dialog");
  await expect(editDialog.getByText("Edit bank account")).toBeVisible();
  await expect(editDialog.getByTestId("tag-select")).toHaveValue(tagId!);

  // --- Clear it in the Edit dialog ---
  const saved = () =>
    page.waitForResponse(
      (r) =>
        /\/api\/bank-accounts\/[^/]+$/.test(r.url()) && r.request().method() === "PATCH" && r.ok(),
    );
  let patched = saved();
  await editDialog.getByTestId("tag-select").selectOption({ label: "No tag" });
  await editDialog.getByRole("button", { name: "Save changes" }).click();
  await patched;
  await expect(editDialog).toHaveCount(0);
  expect(await taggedItemTagId(page, acctName)).toBeNull();

  // --- Set it again in the Edit dialog; persists across a reload ---
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await expect(editDialog.getByTestId("tag-select")).toHaveValue("");
  patched = saved();
  await editDialog.getByTestId("tag-select").selectOption({ label: tagName });
  await editDialog.getByRole("button", { name: "Save changes" }).click();
  await patched;
  await expect(editDialog).toHaveCount(0);
  expect(await taggedItemTagId(page, acctName)).toBe(tagId);

  await page.reload();
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await expect(page.getByRole("dialog").getByTestId("tag-select")).toHaveValue(tagId!);
  await page.getByRole("dialog").getByRole("button", { name: "Cancel" }).click();

  // --- /tags report route renders ---
  await page.goto("/tags");
  await expect(page.getByRole("heading", { level: 1, name: "Tags" })).toBeVisible();

  // --- Cleanup: delete the bank account, then the tag ---
  await page.goto("/assets/bank-accounts");
  await page
    .getByRole("row", { name: new RegExp(acctName) })
    .getByText(acctName)
    .click();
  await page.getByTestId("position-menu-trigger").click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();

  await page.goto("/tags");
  await page.getByTestId(`tag-row-${tagId}`).getByRole("button", { name: "Tag actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: /delete/i })
    .click();
  await expect(page.getByTestId("tag-list").getByText(tagName)).toHaveCount(0);
});

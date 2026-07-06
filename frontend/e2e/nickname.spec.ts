import { test, expect } from "@playwright/test";

// Settings → Nickname round-trip: seeded Alice has nickname NULL, so the input
// renders empty with display_name as placeholder. Set it, save, reload (proves
// persistence past the in-memory React Query cache), then clear it and reload
// again. Self-cleaning so other specs that resolve `ownershipLabel` see the
// seed's NULL state. See ADR-0024.
test("settings nickname round-trip: set, persist, clear", async ({ page }) => {
  await page.goto("/settings");

  const input = page.getByLabel("Nickname");
  await expect(input).toBeVisible();
  await expect(input).toHaveValue("");
  await expect(input).toHaveAttribute("placeholder", "Alice");

  const save = page.getByRole("button", { name: "Save" }).first();

  // --- Set ---
  await input.fill("Ally");
  await expect(save).toBeEnabled();
  await save.click();
  // After the mutation settles, Save returns to disabled (draft cleared, value
  // matches the just-saved nickname).
  await expect(save).toBeDisabled();

  await page.reload();
  await expect(page.getByLabel("Nickname")).toHaveValue("Ally");

  // --- Clear ---
  await page.getByLabel("Nickname").fill("");
  await page.getByRole("button", { name: "Save" }).first().click();
  await expect(page.getByRole("button", { name: "Save" }).first()).toBeDisabled();

  await page.reload();
  await expect(page.getByLabel("Nickname")).toHaveValue("");
});

import { expect, type Page } from "@playwright/test";

// A detail page for a position with a ledger (stock, mutual fund, bond, gold, time
// deposit) tabs its history: Snapshots (n) | Transactions (n) (#839). The inactive
// panel is display:none, so a spec drives the tab it needs before it reaches for a
// control in it. Selecting a tab also writes `?tab=` into the URL.
export async function openHistoryTab(page: Page, tab: "snapshots" | "transactions") {
  const trigger = page.getByTestId(`history-tab-${tab}`);
  await trigger.click();
  await expect(trigger).toHaveAttribute("aria-selected", "true");
}

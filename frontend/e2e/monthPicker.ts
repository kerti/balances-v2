import { expect, type Locator, type Page } from "@playwright/test";

// pickMonth drives a MonthPickerPopover form field: opens the trigger, walks the
// year nav to the target year, then taps the month cell. `trigger` is the
// picker's button (by label or testid); `yearMonth` is "YYYY-MM".
export async function pickMonth(page: Page, trigger: Locator, yearMonth: string) {
  const [year] = yearMonth.split("-").map(Number);
  await trigger.click();
  const content = page.getByTestId("month-picker-content");
  const label = content.getByTestId("month-picker-year-label");
  await expect(label).toBeVisible();
  for (let guard = 0; guard < 60; guard++) {
    const shown = Number(await label.innerText());
    if (shown === year) break;
    await content
      .getByTestId(shown > year ? "month-picker-year-prev" : "month-picker-year-next")
      .click();
  }
  await content.getByTestId(`month-picker-cell-${yearMonth}`).click();
}

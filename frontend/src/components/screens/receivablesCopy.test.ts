// Receivables says "Owed to you" so the direction is never confused with
// Liabilities' "Total owed" (#949).
import { it, expect } from "vitest";
import i18n from "@/i18n";

it("receivables total and chart titles read as owed TO the household", async () => {
  await i18n.changeLanguage("en");
  expect(i18n.t("receivables:totalOutstanding")).toBe("Owed to you");
  expect(i18n.t("receivables:home.valueChartTitle")).toBe("Owed to you over time");
  expect(i18n.t("liabilities:totalOwed")).toBe("Total owed");
});

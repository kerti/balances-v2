import { describe, expect, it } from "vitest";
import type { InvestmentTransaction } from "@/api/types";
import { earliestBuyDate } from "./bondTerm";

function txn(
  transaction_type: InvestmentTransaction["transaction_type"],
  transaction_date: string,
) {
  return { transaction_type, transaction_date } as InvestmentTransaction;
}

// covers: INV-BONDS-05
describe("earliestBuyDate", () => {
  it("is the earliest Buy as a calendar day, whatever the ledger order", () => {
    expect(
      earliestBuyDate([
        txn("coupon", "2025-01-01T00:00:00Z"),
        txn("buy", "2026-03-10T00:00:00Z"),
        txn("buy", "2026-01-15T00:00:00Z"),
        txn("sell", "2025-06-01T00:00:00Z"),
      ]),
    ).toBe("2026-01-15");
  });

  it("is undefined with no Buy (a secondary-market bond before its first Buy) or no ledger", () => {
    expect(earliestBuyDate([txn("coupon", "2026-02-01T00:00:00Z")])).toBeUndefined();
    expect(earliestBuyDate([])).toBeUndefined();
    expect(earliestBuyDate(undefined)).toBeUndefined();
  });
});

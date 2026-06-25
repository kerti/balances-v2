import { describe, it, expect } from "vitest";

import i18n from "@/i18n";
import type { InvestmentTransaction } from "@/api/types";
import { matchesTxnSearch } from "@/lib/transactionSearch";

// A minimal transaction; only transaction_type + description drive the filter.
function txn(
  type: InvestmentTransaction["transaction_type"],
  description: string | null,
): InvestmentTransaction {
  return {
    id: "x",
    investment_id: "y",
    transaction_type: type,
    transaction_date: "2026-01-01",
    currency: "IDR",
    description,
    amount: null,
    quantity: null,
    price_per_unit: null,
    principal_amount: null,
    interest_amount: null,
    principal_disposition: null,
    interest_disposition: null,
    created_by: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_by: null,
    updated_at: "2026-01-01T00:00:00Z",
  };
}

describe("matchesTxnSearch", () => {
  it("matches every transaction on an empty or whitespace query", () => {
    const tx = txn("buy", "anything");
    expect(matchesTxnSearch(tx, "")).toBe(true);
    expect(matchesTxnSearch(tx, "   ")).toBe(true);
  });

  it("matches the localised transaction-type label, case-insensitively", () => {
    const tx = txn("buy", null);
    // The English label for 'buy' is "Buy"; the query is lower-cased.
    expect(i18n.t("investments:transactionType.buy")).toBe("Buy");
    expect(matchesTxnSearch(tx, "buy")).toBe(true);
    expect(matchesTxnSearch(tx, "BU")).toBe(true);
  });

  it("matches the description substring, case-insensitively", () => {
    const tx = txn("sell", "Quarterly REBALANCE");
    expect(matchesTxnSearch(tx, "rebalance")).toBe(true);
    expect(matchesTxnSearch(tx, "quarterly")).toBe(true);
  });

  it("returns false when neither label nor description matches", () => {
    const tx = txn("coupon", "semi-annual payout");
    expect(matchesTxnSearch(tx, "zzz-nomatch")).toBe(false);
  });

  it("handles a null description without matching spuriously", () => {
    const tx = txn("fee", null);
    expect(matchesTxnSearch(tx, "fee")).toBe(true); // matches the type label
    expect(matchesTxnSearch(tx, "note")).toBe(false);
  });
});

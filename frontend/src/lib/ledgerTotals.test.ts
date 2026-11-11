// covers: INV-PRESENTATION-21
import { describe, it, expect } from "vitest";
import { ledgerTotals, LEDGER_TOTAL_KEYS } from "@/lib/ledgerTotals";
import type { InvestmentTransaction, TransactionType } from "@/api/types";

const txn = (
  transaction_type: TransactionType,
  fields: Partial<InvestmentTransaction> = {},
): InvestmentTransaction =>
  ({
    transaction_type,
    currency: "USD",
    amount: null,
    principal_amount: null,
    interest_amount: null,
    ...fields,
  }) as InvestmentTransaction;

describe("which kinds each subtype totals (ADR-0023 subtype -> type matrix)", () => {
  it("lists exactly the recurring/cost kinds each subtype records", () => {
    expect(LEDGER_TOTAL_KEYS).toEqual({
      stock: ["dividend", "fee"],
      mutual_fund: ["distribution", "fee"],
      bond: ["coupon", "fee"],
      gold: ["fee"],
      time_deposit: ["maturityPrincipal", "maturityInterest"],
    });
  });
});

describe("ledgerTotals", () => {
  it("sums exactly where floats would drift", () => {
    const totals = ledgerTotals(
      "stock",
      [
        txn("dividend", { amount: "0.1" }),
        txn("dividend", { amount: "0.2" }),
        txn("fee", { amount: "1.10" }),
        txn("fee", { amount: "2.20" }),
      ],
      "USD",
    );
    expect(totals).toEqual([
      { key: "dividend", amounts: [{ currency: "USD", amount: "0.3" }] },
      { key: "fee", amounts: [{ currency: "USD", amount: "3.3" }] },
    ]);
  });

  it("totals only the subtype's own kinds, ignoring trades and other income", () => {
    const rows = [
      txn("buy", { amount: "5000" }),
      txn("sell", { amount: "900" }),
      txn("coupon", { amount: "70" }),
      txn("dividend", { amount: "150" }),
      txn("fee", { amount: "5" }),
      txn("maturity", { principal_amount: "1000", interest_amount: "40" }),
    ];
    const byKey = (subtype: Parameters<typeof ledgerTotals>[0]) =>
      Object.fromEntries(
        ledgerTotals(subtype, rows, "USD").map((t) => [t.key, t.amounts[0].amount]),
      );
    expect(byKey("stock")).toEqual({ dividend: "150", fee: "5" });
    expect(byKey("mutual_fund")).toEqual({ distribution: "0", fee: "5" });
    expect(byKey("bond")).toEqual({ coupon: "70", fee: "5" });
    expect(byKey("gold")).toEqual({ fee: "5" });
    expect(byKey("time_deposit")).toEqual({ maturityPrincipal: "1000", maturityInterest: "40" });
  });

  it("reports zero in the position's currency for a kind with no rows", () => {
    expect(ledgerTotals("gold", [txn("buy", { amount: "10" })], "IDR")).toEqual([
      { key: "fee", amounts: [{ currency: "IDR", amount: "0" }] },
    ]);
  });

  it("keeps a negative amount negative", () => {
    const [fee] = ledgerTotals(
      "gold",
      [txn("fee", { amount: "-3" }), txn("fee", { amount: "1" })],
      "USD",
    );
    expect(fee.amounts).toEqual([{ currency: "USD", amount: "-2" }]);
  });

  it("never adds across currencies: one figure per currency", () => {
    const [fee] = ledgerTotals(
      "gold",
      [
        txn("fee", { amount: "10", currency: "USD" }),
        txn("fee", { amount: "15000", currency: "IDR" }),
        txn("fee", { amount: "5", currency: "USD" }),
      ],
      "USD",
    );
    expect(fee.amounts).toEqual([
      { currency: "USD", amount: "15" },
      { currency: "IDR", amount: "15000" },
    ]);
  });

  it("ignores an unexpectedly null amount instead of throwing", () => {
    const [fee] = ledgerTotals(
      "gold",
      [txn("fee", { amount: null }), txn("fee", { amount: "2" })],
      "USD",
    );
    expect(fee.amounts).toEqual([{ currency: "USD", amount: "2" }]);
  });
});

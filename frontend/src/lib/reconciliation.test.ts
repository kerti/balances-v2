import { describe, it, expect } from "vitest";
import { ledgerQuantity, reconcileQuantity } from "@/lib/reconciliation";
import type { InvestmentSnapshot, InvestmentTransaction, TransactionType } from "@/api/types";

// reconcileQuantity only reads snapshot.quantity and each transaction's
// type+quantity, so fixtures carry just those fields (cast past the full
// wire types — the rest is irrelevant to the calculation).
const snap = (quantity: string | null): InvestmentSnapshot => ({ quantity }) as InvestmentSnapshot;

const txn = (transaction_type: TransactionType, quantity: string | null): InvestmentTransaction =>
  ({ transaction_type, quantity }) as InvestmentTransaction;

describe("reconcileQuantity", () => {
  it("returns null when there is no snapshot to compare against", () => {
    expect(reconcileQuantity(null, [txn("buy", "100")])).toBeNull();
    expect(reconcileQuantity(undefined, [txn("buy", "100")])).toBeNull();
  });

  it("returns null for the accrued-interest shape (snapshot has no quantity)", () => {
    expect(reconcileQuantity(snap(null), [txn("buy", "100")])).toBeNull();
  });

  it("returns null when there are no transactions to total", () => {
    expect(reconcileQuantity(snap("100"), undefined)).toBeNull();
    expect(reconcileQuantity(snap("100"), [])).toBeNull();
  });

  it("sums buys and subtracts sells and fees", () => {
    const result = reconcileQuantity(snap("70"), [
      txn("buy", "100"),
      txn("sell", "20"),
      txn("fee", "10"),
    ]);
    expect(result).toEqual({ expected: 70, actual: 70, matches: true });
  });

  it("flags a mismatch when the ledger total disagrees with the snapshot", () => {
    const result = reconcileQuantity(snap("90"), [txn("buy", "100")]);
    expect(result).toMatchObject({ expected: 100, actual: 90, matches: false });
  });

  it("ignores cash-only types and transactions carrying no quantity", () => {
    const result = reconcileQuantity(snap("100"), [
      txn("buy", "100"),
      txn("dividend", null),
      txn("coupon", null),
      txn("buy", null), // malformed buy contributes nothing
    ]);
    expect(result).toMatchObject({ expected: 100, actual: 100, matches: true });
  });

  it("treats sub-1e-6 drift as a match", () => {
    const result = reconcileQuantity(snap("100.0000001"), [txn("buy", "100")]);
    expect(result?.matches).toBe(true);
  });
});

describe("ledgerQuantity", () => {
  it("is null with no ledger", () => {
    expect(ledgerQuantity(undefined)).toBeNull();
    expect(ledgerQuantity([])).toBeNull();
  });

  it("totals buys less sells and fee-deducted units, exactly", () => {
    expect(ledgerQuantity([txn("buy", "40"), txn("buy", "10")])).toBe("50");
    expect(ledgerQuantity([txn("buy", "100"), txn("sell", "20"), txn("fee", "10")])).toBe("70");
    expect(ledgerQuantity([txn("buy", "0.1"), txn("buy", "0.2")])).toBe("0.3");
    expect(0.1 + 0.2).not.toBe(0.3);
  });

  it("can reach zero or go negative, and ignores income and null quantities", () => {
    expect(ledgerQuantity([txn("buy", "5"), txn("sell", "5")])).toBe("0");
    expect(ledgerQuantity([txn("sell", "2.5")])).toBe("-2.5");
    expect(ledgerQuantity([txn("buy", null), txn("dividend", null), txn("buy", "3")])).toBe("3");
  });
});

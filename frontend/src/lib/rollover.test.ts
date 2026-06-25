import { describe, it, expect } from "vitest";
import { addMonths, maturityRolloverPrefill } from "@/lib/rollover";
import type {
  Disposition,
  InvestmentTransaction,
  TimeDeposit,
} from "@/api/types";

const maturity = (fields: {
  date?: string;
  principal?: string | null;
  interest?: string | null;
  principalDisp?: Disposition | null;
  interestDisp?: Disposition | null;
}): InvestmentTransaction =>
  ({
    transaction_type: "maturity",
    transaction_date: fields.date ?? "2026-06-01",
    principal_amount: fields.principal ?? null,
    interest_amount: fields.interest ?? null,
    principal_disposition: fields.principalDisp ?? null,
    interest_disposition: fields.interestDisp ?? null,
  }) as InvestmentTransaction;

const td = (hasSuccessor = false): TimeDeposit =>
  ({
    rolled_from: null,
    rolled_to: hasSuccessor
      ? { id: "succ-1", display_name: "BCA 6mo (rolled)" }
      : null,
    investment: {
      display_name: "BCA 6mo",
      description: "Emergency fund deposit",
      ownership_type: "joint",
      sole_owner_user_id: null,
      risk_profile: "low",
      native_currency: "IDR",
    },
    details: {
      bank_name: "BCA",
      principal: "100000000",
      interest_rate: "5.5",
      term_months: 6,
      placement_date: "2025-12-01T00:00:00Z",
      // RFC3339 wire shape (Go time.Time) — the helper must slice to YYYY-MM-DD.
      maturity_date: "2026-06-01T00:00:00Z",
      rollover_policy: "auto_renew_with_interest",
    },
  }) as TimeDeposit;

describe("addMonths", () => {
  it("adds whole months", () => {
    expect(addMonths("2026-06-01", 6)).toBe("2026-12-01");
  });
  it("returns empty for missing or non-positive inputs", () => {
    expect(addMonths("", 6)).toBe("");
    expect(addMonths("2026-06-01", 0)).toBe("");
    expect(addMonths("2026-06-01", Number.NaN)).toBe("");
    expect(addMonths("not-a-date", 6)).toBe("");
  });
});

describe("maturityRolloverPrefill", () => {
  it("returns null when there is no maturity transaction", () => {
    expect(maturityRolloverPrefill(td(), [])).toBeNull();
    expect(maturityRolloverPrefill(td(), undefined)).toBeNull();
  });

  it("returns null when a rollover successor already exists (issue #29)", () => {
    expect(
      maturityRolloverPrefill(td(true), [
        maturity({
          principal: "100000000",
          interest: "5000000",
          principalDisp: "rolled_to_new",
          interestDisp: "rolled_to_new",
        }),
      ]),
    ).toBeNull();
  });

  it("returns null when everything was cashed out", () => {
    expect(
      maturityRolloverPrefill(td(), [
        maturity({
          principal: "100000000",
          interest: "5000000",
          principalDisp: "cash_out",
          interestDisp: "cash_out",
        }),
      ]),
    ).toBeNull();
  });

  it("rolls principal + interest when both rolled", () => {
    const r = maturityRolloverPrefill(td(), [
      maturity({
        // Bank settled the maturity txn a day late; placement still tracks the
        // TD's scheduled maturity_date, not this transaction_date.
        date: "2026-06-02",
        principal: "100000000",
        interest: "5000000",
        principalDisp: "rolled_to_new",
        interestDisp: "rolled_to_new",
      }),
    ]);
    expect(r?.rolledAmount).toBe(105000000);
    expect(r?.prefill.principal).toBe("105000000");
    // placement_date = the TD's maturity date; new maturity recomputed from term.
    expect(r?.prefill.placement_date).toBe("2026-06-01");
    expect(r?.prefill.maturity_date).toBe("2026-12-01");
    expect(r?.prefill.description).toBe("Emergency fund deposit");
    expect(r?.prefill.bank_name).toBe("BCA");
    expect(r?.prefill.interest_rate).toBe("5.5");
    expect(r?.prefill.term_months).toBe("6");
    expect(r?.prefill.rollover_policy).toBe("auto_renew_with_interest");
    expect(r?.prefill.risk_profile).toBe("low");
  });

  it("rolls only the principal when interest was cashed out", () => {
    const r = maturityRolloverPrefill(td(), [
      maturity({
        principal: "100000000",
        interest: "5000000",
        principalDisp: "rolled_to_new",
        interestDisp: "cash_out",
      }),
    ]);
    expect(r?.rolledAmount).toBe(100000000);
    expect(r?.prefill.principal).toBe("100000000");
  });

  it("rolls only the interest when principal was cashed out", () => {
    const r = maturityRolloverPrefill(td(), [
      maturity({
        principal: "100000000",
        interest: "5000000",
        principalDisp: "cash_out",
        interestDisp: "rolled_to_new",
      }),
    ]);
    expect(r?.rolledAmount).toBe(5000000);
    expect(r?.prefill.principal).toBe("5000000");
  });

  it("short-circuits when a successor deposit already exists (issue #29)", () => {
    const r = maturityRolloverPrefill(td(true), [
      maturity({
        principal: "100000000",
        interest: "5000000",
        principalDisp: "rolled_to_new",
        interestDisp: "rolled_to_new",
      }),
    ]);
    expect(r).toBeNull();
  });

  it("returns null when there is no maturity transaction", () => {
    expect(maturityRolloverPrefill(td(), [])).toBeNull();
    expect(maturityRolloverPrefill(td(), undefined)).toBeNull();
  });

  it("returns null when both principal and interest were cashed out", () => {
    const r = maturityRolloverPrefill(td(), [
      maturity({
        principal: "100000000",
        interest: "5000000",
        principalDisp: "cash_out",
        interestDisp: "cash_out",
      }),
    ]);
    expect(r).toBeNull();
  });

  it("treats null rolled amounts as zero (nullish fallback)", () => {
    // Disposition says rolled, but the amount fields are null — coalesces to 0,
    // so nothing rolls and the helper returns null.
    const r = maturityRolloverPrefill(td(), [
      maturity({
        principal: null,
        interest: null,
        principalDisp: "rolled_to_new",
        interestDisp: "rolled_to_new",
      }),
    ]);
    expect(r).toBeNull();
  });

  it("defaults a null position description to an empty string in the prefill", () => {
    const base = td();
    const noDesc = {
      ...base,
      investment: { ...base.investment, description: null },
    } as TimeDeposit;
    const r = maturityRolloverPrefill(noDesc, [
      maturity({ principal: "100000000", principalDisp: "rolled_to_new" }),
    ]);
    expect(r?.prefill.description).toBe("");
  });
});

// The investment card's money summary builder (#994): the order Value -> Cost ->
// Gain / loss -> income received, the income row's per-type label and totals, and
// the Gain / loss tooltip's "leaves out income" clause.
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { createInstance } from "i18next";
import en from "@/locales/en/investments.json";
import { investmentHeadlineFields } from "./investmentHeadlineFields";
import type { InvestmentTransaction } from "@/api/types";

const i18n = createInstance();
void i18n.init({
  lng: "en",
  resources: { en: { investments: en } },
  interpolation: { escapeValue: false },
});
const t = i18n.getFixedT("en");

function tx(type: string, amount: string, currency = "USD"): InvestmentTransaction {
  return {
    id: `${type}-${amount}-${currency}`,
    investment_id: "i1",
    transaction_type: type,
    transaction_date: "2026-06-15T00:00:00Z",
    currency,
    description: null,
    amount,
    quantity: null,
    price_per_unit: null,
    principal_amount: null,
    interest_amount: type === "maturity" ? amount : null,
    principal_disposition: null,
    interest_disposition: null,
    created_by: "u1",
    created_at: "2026-06-15T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-06-15T00:00:00Z",
  } as InvestmentTransaction;
}

const base = { currency: "USD", latestValue: 1100, totalCost: 1000 };

describe("investmentHeadlineFields", () => {
  // covers: INV-PRESENTATION-38
  it.each([
    ["bond", "Coupons earned"],
    ["stock", "Dividends received"],
    ["mutual_fund", "Distributions received"],
    ["time_deposit", "Interest received"],
  ] as const)("%s: value, cost, gain / loss, then %s", (subtype, income) => {
    const status = subtype === "time_deposit" ? "matured" : "active";
    const terminatedAt = subtype === "time_deposit" ? "2026-06-15" : null;
    const fields = investmentHeadlineFields(
      { ...base, subtype, status, terminatedAt, transactions: [] },
      t,
    );
    expect(fields.map((f) => f.label)).toEqual([
      "Total value",
      "Total cost",
      subtype === "time_deposit" ? "Matured on" : "Gain / loss",
      income,
    ]);
  });

  // covers: INV-PRESENTATION-38
  it("an active time deposit has no interest row until it matures", () => {
    const fields = investmentHeadlineFields(
      { ...base, subtype: "time_deposit", status: "active", transactions: [] },
      t,
    );
    expect(fields.map((f) => f.label)).toEqual(["Total value", "Total cost", "Gain / loss"]);
    expect(fields[2].hint).not.toContain("leaves out");
  });

  // covers: INV-PRESENTATION-38
  it("gold has no income row", () => {
    const fields = investmentHeadlineFields({ ...base, subtype: "gold", transactions: [] }, t);
    expect(fields.map((f) => f.label)).toEqual(["Total value", "Total cost", "Gain / loss"]);
  });

  // covers: INV-PRESENTATION-38
  it("totals the ledger's income, and shows 0 in the position currency when empty", () => {
    const withRows = investmentHeadlineFields(
      {
        ...base,
        subtype: "stock",
        transactions: [tx("dividend", "10.50"), tx("dividend", "4.50"), tx("fee", "99")],
      },
      t,
    );
    render(<>{withRows[3].value}</>);
    expect(screen.getByTestId("investment-headline-income")).toHaveTextContent("15.00");

    const empty = investmentHeadlineFields({ ...base, subtype: "stock", transactions: [] }, t);
    render(<>{empty[3].value}</>);
    expect(screen.getAllByTestId("investment-headline-income")[1]).toHaveTextContent("0.00");
  });

  // covers: INV-PRESENTATION-38
  it("keeps one figure per currency, like the ledger toolbar", () => {
    const fields = investmentHeadlineFields(
      {
        ...base,
        subtype: "bond",
        transactions: [tx("coupon", "10", "USD"), tx("coupon", "20", "EUR")],
      },
      t,
    );
    render(<>{fields[3].value}</>);
    const text = screen.getByTestId("investment-headline-income").textContent ?? "";
    expect(text).toContain("·");
    expect(text).toMatch(/10\.00/);
    expect(text).toMatch(/20\.00/);
  });

  // covers: INV-PRESENTATION-38
  it("Gain / loss hint says income is left out, per type; gold's has no clause", () => {
    const hint = (subtype: "bond" | "gold") =>
      investmentHeadlineFields({ ...base, subtype, transactions: [] }, t)[2].hint;
    expect(hint("bond")).toContain("leaves out coupons earned");
    expect(hint("gold")).not.toContain("leaves out");
  });
});

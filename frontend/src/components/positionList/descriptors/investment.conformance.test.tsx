// Per-type conformance for the remaining investment descriptors (ADR-0043).
// One table-driven pass over Stock / MutualFund / Bond / TimeDeposit on the
// investment preset: each renders through the generic core over MSW, loads its
// list, and surfaces its subtype column + the cost/P-L headline. Gold (the
// preset's tracer) has its own suite; core list behaviour is proven generically
// in PositionListScreen.test.tsx.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse, type HttpHandler } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionListScreen } from "@/components/positionList/PositionListScreen";
import { stockDescriptor } from "./stock";
import { mutualFundDescriptor } from "./mutualFund";
import { bondDescriptor } from "./bond";
import { timeDepositDescriptor } from "./timeDeposit";
import type { PositionListDescriptor } from "@/components/positionList/types";
import type { Investment, InvestmentSnapshot } from "@/api/types";

function investment(name: string): Investment {
  return {
    id: "i1",
    household_id: "h1",
    display_name: name,
    description: null,
    subtype: "stock",
    ownership_type: "sole",
    sole_owner_user_id: "u1",
    native_currency: "USD",
    risk_profile: "high",
    rolled_from_investment_id: null,
    tag_id: null,
    status: "active",
    terminated_at: null,
    termination_note: null,
    created_by: "u1",
    created_at: "2026-01-01T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

const snapshot: InvestmentSnapshot = {
  id: "s1",
  investment_id: "i1",
  year_month: "2026-06-01T00:00:00Z",
  amount: "4321.00",
  currency: "USD",
  quantity: null,
  price_per_unit: null,
  accrued_interest: null,
  as_of_date: null,
  description: null,
  created_by: "u1",
  created_at: "2026-06-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-06-01T00:00:00Z",
};

// The list-payload extras every investment row carries (issues #18/#67).
const ledger = {
  latest_snapshot: snapshot,
  cost_basis: "4000.00",
  transaction_count: 3,
  last_transaction_date: "2026-05-01T00:00:00Z",
};

const stockItem = {
  investment: investment("Big Co"),
  details: { ticker: "BIGC", exchange: "NASDAQ" },
  ...ledger,
};
const mutualFundItem = {
  investment: investment("Growth Fund"),
  details: { fund_code: "GRW01", fund_manager: "Acme AM", fund_type: "equity" },
  ...ledger,
};
const bondItem = {
  investment: investment("Govt Bond"),
  details: {
    bond_type: "govt_primary",
    series_code: "GB01",
    issuer: "Treasury",
    coupon_rate: "5.00",
    coupon_frequency: "annual",
    coupon_disposition: "pays_out",
    maturity_date: "2030-01-01T00:00:00Z",
  },
  ...ledger,
};
const timeDepositItem = {
  investment: investment("Term Deposit"),
  details: {
    bank_name: "Test Bank",
    principal: "4000.00",
    interest_rate: "4.00",
    term_months: 12,
    placement_date: "2026-01-01T00:00:00Z",
    maturity_date: "2030-01-01T00:00:00Z",
    rollover_policy: "auto_renew_principal",
  },
  ...ledger,
};

type Case = {
  label: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  descriptor: PositionListDescriptor<any, any>;
  rowTestId: string;
  headlineTestId: string;
  name: RegExp;
  subtype: RegExp;
  path: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  item: any;
};

const cases: Case[] = [
  {
    label: "stock",
    descriptor: stockDescriptor,
    rowTestId: "stock-row",
    headlineTestId: "stocks-total",
    name: /Big Co/,
    subtype: /BIGC/,
    path: "/api/investments/stocks",
    item: stockItem,
  },
  {
    label: "mutual fund",
    descriptor: mutualFundDescriptor,
    rowTestId: "mutual-fund-row",
    headlineTestId: "mutual-funds-total",
    name: /Growth Fund/,
    subtype: /GRW01/,
    path: "/api/investments/mutual-funds",
    item: mutualFundItem,
  },
  {
    label: "bond",
    descriptor: bondDescriptor,
    rowTestId: "bond-row",
    headlineTestId: "bonds-total",
    name: /Govt Bond/,
    subtype: /GB01/,
    path: "/api/investments/bonds",
    item: bondItem,
  },
  {
    label: "time deposit",
    descriptor: timeDepositDescriptor,
    rowTestId: "time-deposit-row",
    headlineTestId: "time-deposits-total",
    name: /Term Deposit/,
    subtype: /Test Bank/,
    path: "/api/investments/time-deposits",
    item: timeDepositItem,
  },
];

describe("investment descriptors (conformance)", () => {
  it.each(cases)(
    "$label loads and surfaces its subtype column + headline",
    async ({
      descriptor,
      rowTestId,
      headlineTestId,
      name,
      subtype,
      path,
      item,
    }) => {
      const handlers: HttpHandler[] = [
        http.get(path, () => HttpResponse.json([item])),
        http.get("/api/investments/time-series", () => HttpResponse.json([])),
      ];
      server.use(...handlers);
      renderWithProviders(
        <PositionListScreen descriptor={descriptor} onSelect={vi.fn()} />,
      );

      const row = await screen.findByTestId(rowTestId);
      expect(within(row).getByText(name)).toBeInTheDocument();
      expect(within(row).getByText(subtype)).toBeInTheDocument();
      // Shared surface + activity column.
      expect(within(row).getByText("Active")).toBeInTheDocument();
      expect(within(row).getByText(/4,?321/)).toBeInTheDocument();
      expect(within(row).getByText("3 transactions")).toBeInTheDocument();
      // The investment cost/P-L headline slot.
      expect(screen.getByTestId(headlineTestId)).toBeInTheDocument();
    },
  );
});

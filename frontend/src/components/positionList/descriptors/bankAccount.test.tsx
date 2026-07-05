// Per-type conformance for the BankAccount descriptor (ADR-0043). It drives the
// generic core with the *real* descriptor over MSW-stubbed endpoints and
// asserts the descriptor wires up: its list hook loads, its declared columns
// (name+secondary, ownership, status, latest balance) render, and the headline
// slot shows. This is the cheap per-type net — core list *behaviour* is proven
// once, generically, in PositionListScreen.test.tsx.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionListScreen } from "@/components/positionList/PositionListScreen";
import { bankAccountDescriptor } from "./bankAccount";
import type {
  Asset,
  AssetSnapshot,
  BankAccountDetails,
  BankAccountListItem,
  HouseholdMember,
} from "@/api/types";
import type { Me } from "@/hooks/useSession";

const owner: HouseholdMember = {
  id: "u1",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
};

const me: Me = {
  id: "u1",
  household_id: "h1",
  household_display_name: "Test Household",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
  picture_url: null,
  locale: "en-GB",
  theme: "system",
  carryover_date_mode: "month_end",
  time_zone: "UTC",
  reporting_currency: "USD",
  multi_currency_enabled: false,
  is_founder: true,
};

const asset: Asset = {
  id: "a1",
  household_id: "h1",
  display_name: "Everyday Checking",
  description: null,
  subtype: "bank_account",
  ownership_type: "sole",
  sole_owner_user_id: "u1",
  native_currency: "USD",
  tag_id: null,
  status: "active",
  terminated_at: null,
  termination_note: null,
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
};

const details: BankAccountDetails = {
  asset_id: "a1",
  bank_name: "Test Bank",
  account_number: "1234567890",
  account_type: "savings",
};

const snapshot: AssetSnapshot = {
  id: "s1",
  asset_id: "a1",
  year_month: "2026-06-01T00:00:00Z",
  amount: "4321.00",
  currency: "USD",
  as_of_date: null,
  description: null,
  created_by: "u1",
  created_at: "2026-06-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-06-01T00:00:00Z",
};

const listItem: BankAccountListItem = {
  asset,
  details,
  latest_snapshot: snapshot,
};

function stubEndpoints() {
  server.use(
    http.get("/api/bank-accounts", () => HttpResponse.json([listItem])),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("bankAccountDescriptor (conformance)", () => {
  it("loads the list and renders every declared column", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionListScreen
        descriptor={bankAccountDescriptor}
        onSelect={vi.fn()}
      />,
    );

    // List hook resolved through MSW → the row is here.
    const row = await screen.findByTestId("bank-account-row");

    // Shared surface: name + the bank-detail secondary line + status + value.
    expect(within(row).getByText("Everyday Checking")).toBeInTheDocument();
    expect(
      within(row).getByText(/Test Bank · 1234567890 · Savings/),
    ).toBeInTheDocument();
    expect(within(row).getByText("Active")).toBeInTheDocument();
    expect(within(row).getByText(/4,?321/)).toBeInTheDocument();

    // Ownership extra column: privacy-safe sole-owner label with "(you)".
    expect(within(row).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();

    // Declared headers + headline slot.
    expect(
      screen.getByRole("columnheader", { name: /Ownership/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: /Latest balance/ }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("bank-accounts-total")).toBeInTheDocument();
  });
});

// Per-type conformance for the non-investment descriptors (ADR-0043). One
// table-driven pass over every real descriptor on the non-investment preset:
// each renders through the generic core over MSW-stubbed endpoints without
// crashing, loads its list, and surfaces the shared surface + its ownership
// column + headline. Core list *behaviour* is proven generically in
// PositionListScreen.test.tsx; this only checks each descriptor is wired right.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse, type HttpHandler } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionListScreen } from "@/components/positionList/PositionListScreen";
import { propertyDescriptor } from "./property";
import { vehicleDescriptor } from "./vehicle";
import { liabilityPersonalDescriptor } from "./liability";
import { receivableDescriptor } from "./receivable";
import type { PositionListDescriptor } from "@/components/positionList/types";
import type {
  Asset,
  AssetSnapshot,
  HouseholdMember,
  Liability,
  LiabilitySnapshot,
  PropertyListItem,
  Receivable,
  ReceivableSnapshot,
  VehicleListItem,
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

// Endpoints every non-investment list shares (ownership context).
const commonHandlers: HttpHandler[] = [
  http.get("/api/household/members", () => HttpResponse.json([owner])),
  http.get("/api/me", () => HttpResponse.json(me)),
];

function baseAsset(subtype: Asset["subtype"]): Asset {
  return {
    id: "a1",
    household_id: "h1",
    display_name: "Test Position",
    description: null,
    subtype,
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
}

function assetSnapshot(): AssetSnapshot {
  return {
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
}

const propertyItem: PropertyListItem = {
  asset: { ...baseAsset("property"), display_name: "Lake House" },
  details: {
    asset_id: "a1",
    property_type: "house",
    address: "1 Main St",
    acquisition_date: null,
    acquisition_cost: null,
    annual_appreciation_rate: null,
  },
  latest_snapshot: assetSnapshot(),
};

const vehicleItem: VehicleListItem = {
  asset: { ...baseAsset("vehicle"), display_name: "Family Car" },
  details: {
    asset_id: "a1",
    vehicle_type: "car",
    make: "Toyota",
    model: "Corolla",
    year: 2020,
    plate_number: "ABC123",
    annual_depreciation_rate: null,
  },
  latest_snapshot: assetSnapshot(),
};

const liability: Liability = {
  id: "l1",
  household_id: "h1",
  display_name: "Home Loan",
  description: null,
  subtype: "personal",
  ownership_type: "sole",
  sole_owner_user_id: "u1",
  native_currency: "USD",
  tag_id: null,
  status: "active",
  terminated_at: null,
  termination_note: null,
  counterparty_name: "Test Bank",
  principal: null,
  interest_rate: null,
  term_months: null,
  start_date: null,
  maturity_date: null,
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
};

const liabilitySnapshot: LiabilitySnapshot = {
  id: "ls1",
  liability_id: "l1",
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

const receivable: Receivable = {
  id: "r1",
  household_id: "h1",
  display_name: "Loan to Friend",
  description: null,
  ownership_type: "sole",
  sole_owner_user_id: "u1",
  native_currency: "USD",
  tag_id: null,
  status: "active",
  terminated_at: null,
  termination_note: null,
  counterparty_name: "A Friend",
  due_date: null,
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
};

const receivableSnapshot: ReceivableSnapshot = {
  id: "rs1",
  receivable_id: "r1",
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

type Case = {
  label: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  descriptor: PositionListDescriptor<any, any>;
  rowTestId: string;
  headlineTestId: string;
  name: RegExp;
  secondary: RegExp;
  handlers: HttpHandler[];
};

const cases: Case[] = [
  {
    label: "property",
    descriptor: propertyDescriptor,
    rowTestId: "property-row",
    headlineTestId: "properties-total",
    name: /Lake House/,
    secondary: /House · 1 Main St/,
    handlers: [http.get("/api/properties", () => HttpResponse.json([propertyItem]))],
  },
  {
    label: "vehicle",
    descriptor: vehicleDescriptor,
    rowTestId: "vehicle-row",
    headlineTestId: "vehicles-total",
    name: /Family Car/,
    secondary: /Car · Toyota Corolla · 2020 · ABC123/,
    handlers: [http.get("/api/vehicles", () => HttpResponse.json([vehicleItem]))],
  },
  {
    label: "liability (personal)",
    descriptor: liabilityPersonalDescriptor,
    rowTestId: "liability-row",
    headlineTestId: "liabilities-total",
    name: /Home Loan/,
    secondary: /Test Bank/,
    handlers: [
      http.get("/api/liabilities", () =>
        HttpResponse.json([{ liability, latest_snapshot: liabilitySnapshot }]),
      ),
    ],
  },
  {
    label: "receivable",
    descriptor: receivableDescriptor,
    rowTestId: "receivable-row",
    headlineTestId: "receivables-total",
    name: /Loan to Friend/,
    secondary: /A Friend/,
    handlers: [
      http.get("/api/receivables", () =>
        HttpResponse.json([{ receivable, latest_snapshot: receivableSnapshot }]),
      ),
      http.get("/api/receivables/time-series", () => HttpResponse.json([])),
    ],
  },
];

describe("non-investment descriptors (conformance)", () => {
  it.each(cases)(
    "$label loads and surfaces its declared columns",
    async ({ descriptor, rowTestId, headlineTestId, name, secondary, handlers }) => {
      server.use(...commonHandlers, ...handlers);
      renderWithProviders(<PositionListScreen descriptor={descriptor} onSelect={vi.fn()} />);

      const row = await screen.findByTestId(rowTestId);
      expect(within(row).getByText(name)).toBeInTheDocument();
      expect(within(row).getByText(secondary)).toBeInTheDocument();
      // Shared surface: status + value.
      expect(within(row).getByText("Active")).toBeInTheDocument();
      expect(within(row).getByText(/4,?321/)).toBeInTheDocument();
      // Ownership extra column: privacy-safe sole-owner label.
      expect(within(row).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
      // Declared ownership header + the headline slot.
      expect(screen.getByRole("columnheader", { name: /Ownership/ })).toBeInTheDocument();
      expect(screen.getByTestId(headlineTestId)).toBeInTheDocument();
    },
  );
});

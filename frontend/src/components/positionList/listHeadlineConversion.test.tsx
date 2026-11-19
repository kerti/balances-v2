// A position list's headline in the reporting currency (#937, ADR-0022
// amendment): for a multi-currency household holding a foreign currency it is the
// group's net-worth share straight from the dashboard's engine pass
// (/api/reports/groups/{group}) — by_subtype[subtype] for a subtype list, total
// for a whole-group list — with the missing-rate warning narrowed to the list's
// positions. Every other household keeps today's native headline.
// covers: INV-FINANCE-39
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionListScreen } from "@/components/positionList/PositionListScreen";
import { propertyDescriptor } from "@/components/positionList/descriptors/property";
import { receivableDescriptor } from "@/components/positionList/descriptors/receivable";
import { timeDepositDescriptor } from "@/components/positionList/descriptors/timeDeposit";
import { formatCurrency } from "@/lib/format";
import type { PositionListDescriptor } from "@/components/positionList/types";
import type { GroupBreakdown, GroupBreakdownMonth } from "@/api/types";

const flat = (s: string | null | undefined) => (s ?? "").replace(/\u00a0/g, " ");
const money = (amount: string, currency: string) => flat(formatCurrency(amount, currency));

const me = (multi: boolean) => ({
  id: "u1",
  household_id: "h1",
  reporting_currency: "IDR",
  multi_currency_enabled: multi,
  locale: "en-GB",
  time_zone: "UTC",
});

function month(
  ym: string,
  total: string,
  bySubtype: Record<string, string>,
  missing: GroupBreakdownMonth["missing_fx"] = [],
): GroupBreakdownMonth {
  return {
    year_month: `${ym}-01T00:00:00Z`,
    total,
    by_subtype: bySubtype,
    missing_fx: missing,
  };
}

const breakdown = (
  group: GroupBreakdown["group"],
  months: GroupBreakdownMonth[],
): GroupBreakdown => ({
  group,
  reporting_currency: "IDR",
  months,
});

const snap = (amount: string, currency: string) => ({
  amount,
  currency,
  year_month: "2026-03-01T00:00:00Z",
});

const owner = { id: "u1", display_name: "Pat", nickname: null, email: "pat@example.test" };

const position = (id: string, currency: string) => ({
  id,
  display_name: `Pos ${id}`,
  description: null,
  ownership_type: "sole",
  sole_owner_user_id: "u1",
  native_currency: currency,
  status: "active",
  terminated_at: null,
});

const propertyItem = (id: string, currency: string, amount: string) => ({
  asset: { ...position(id, currency), subtype: "property" },
  details: { property_type: "house", address: "1 Main St" },
  latest_snapshot: snap(amount, currency),
});

const receivableItem = (id: string, currency: string, amount: string) => ({
  receivable: { ...position(id, currency), counterparty_name: "A Friend", due_date: null },
  latest_snapshot: snap(amount, currency),
});

const timeDepositItem = (id: string, currency: string, amount: string) => ({
  investment: { ...position(id, currency), subtype: "time_deposit", risk_profile: "low" },
  details: {
    bank_name: "Test Bank",
    principal: amount,
    interest_rate: "4.00",
    term_months: 12,
    placement_date: "2026-01-01T00:00:00Z",
    maturity_date: "2030-01-01T00:00:00Z",
    rollover_policy: "auto_renew_principal",
  },
  latest_snapshot: snap(amount, currency),
  cost_basis: amount,
  transaction_count: 0,
  last_transaction_date: null,
});

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function render(descriptor: PositionListDescriptor<any, any>) {
  return renderWithProviders(
    <MemoryRouter>
      <PositionListScreen descriptor={descriptor} onSelect={vi.fn()} />
    </MemoryRouter>,
  );
}

function stubCommon(multi: boolean) {
  server.use(
    http.get("/api/me", () => HttpResponse.json(me(multi))),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/receivables/time-series", () => HttpResponse.json([])),
    http.get("/api/investments/time-series", () => HttpResponse.json([])),
  );
}

describe("list headline in the reporting currency", () => {
  it("a subtype list headlines by_subtype[subtype], not a sum of its native rows", async () => {
    stubCommon(true);
    server.use(
      http.get("/api/properties", () =>
        HttpResponse.json([propertyItem("p1", "IDR", "2000000"), propertyItem("p2", "USD", "100")]),
      ),
      http.get("/api/reports/groups/asset", () =>
        HttpResponse.json(
          breakdown("asset", [
            month("2026-03", "3600000", { bank_account: "1000000", property: "2600000" }),
          ]),
        ),
      ),
    );
    render(propertyDescriptor);

    const headline = await screen.findByTestId("properties-total");
    expect(flat(screen.getByTestId("home-total-value").textContent)).toBe(money("2600000", "IDR"));
    expect(flat(screen.getByTestId("home-total-native").textContent)).toContain(
      `incl. ${money("100", "USD")}`,
    );
    expect(headline).toBeInTheDocument();
    expect(screen.queryByTestId("missing-fx-warning")).not.toBeInTheDocument();
  });

  it("a whole-group list (receivables) headlines the group total", async () => {
    stubCommon(true);
    server.use(
      http.get("/api/receivables", () => HttpResponse.json([receivableItem("r1", "USD", "100")])),
      http.get("/api/reports/groups/receivable", () =>
        HttpResponse.json(breakdown("receivable", [month("2026-03", "1600000", {})])),
      ),
    );
    render(receivableDescriptor);

    await screen.findByTestId("receivables-total");
    expect(flat(screen.getByTestId("home-total-value").textContent)).toBe(money("1600000", "IDR"));
  });

  it("an investment list shows the converted value with cost and P/L still native", async () => {
    stubCommon(true);
    server.use(
      http.get("/api/investments/time-deposits", () =>
        HttpResponse.json([timeDepositItem("t1", "USD", "100")]),
      ),
      http.get("/api/reports/groups/investment", () =>
        HttpResponse.json(
          breakdown("investment", [month("2026-03", "9000000", { time_deposit: "1600000" })]),
        ),
      ),
    );
    render(timeDepositDescriptor);

    await screen.findByTestId("time-deposits-total");
    expect(flat(screen.getByTestId("home-total-value").textContent)).toBe(money("1600000", "IDR"));
    const headline = screen.getByTestId("time-deposits-total");
    expect(flat(headline.textContent)).toContain(money("100", "USD"));
  });

  it("warns only about this list's positions", async () => {
    stubCommon(true);
    server.use(
      http.get("/api/properties", () =>
        HttpResponse.json([propertyItem("p1", "IDR", "2000000"), propertyItem("p2", "USD", "100")]),
      ),
      http.get("/api/reports/groups/asset", () =>
        HttpResponse.json(
          breakdown("asset", [
            month("2026-03", "2000000", { property: "2000000" }, [
              { position_id: "p2", currency: "USD" },
              { position_id: "other-list-position", currency: "EUR" },
            ]),
          ]),
        ),
      ),
    );
    render(propertyDescriptor);

    const warning = await screen.findByTestId("missing-fx-warning");
    expect(warning).toHaveTextContent(/1 item/);
    expect(warning).toHaveTextContent(/USD/);
    expect(warning).not.toHaveTextContent(/EUR/);
  });

  it("keeps the native per-currency headline for a single-currency household", async () => {
    stubCommon(false);
    server.use(
      http.get("/api/properties", () => HttpResponse.json([propertyItem("p1", "IDR", "2000000")])),
    );
    let asked = false;
    server.use(
      http.get("/api/reports/groups/asset", () => {
        asked = true;
        return HttpResponse.json(breakdown("asset", []));
      }),
    );
    render(propertyDescriptor);

    const headline = await screen.findByTestId("properties-total");
    expect(flat(headline.textContent)).toContain(money("2000000", "IDR"));
    expect(screen.queryByTestId("home-total-value")).not.toBeInTheDocument();
    expect(asked).toBe(false);
  });
});

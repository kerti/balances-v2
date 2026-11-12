// The overview hubs in the reporting currency (#845, ADR-0022 amendment): a
// multi-currency household holding a foreign currency sees one converted card
// set fed by the dashboard's own engine pass (/api/reports/groups/{group}), a
// muted native-subtotals line, the dashboard's missing-rate warning, and no
// composition chart with a single category.
// covers: INV-FINANCE-39
import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { AssetsHome } from "@/components/screens/AssetsHome";
import { LiabilitiesHome } from "@/components/screens/LiabilitiesHome";
import { InvestmentsHome } from "@/components/screens/InvestmentsHome";
import { formatCurrency } from "@/lib/format";
import type { GroupBreakdown, GroupBreakdownMonth } from "@/api/types";

// Intl puts a no-break space between a currency code and its number; compare
// with it flattened so the assertions read as the copy does.
const flat = (s: string | null | undefined) => (s ?? "").replace(/\u00a0/g, " ");
const money = (amount: string, currency: string) => flat(formatCurrency(amount, currency));

const me = (multi: boolean) => ({ reporting_currency: "IDR", multi_currency_enabled: multi });

function month(
  ym: string,
  total: string,
  bySubtype: Record<string, string>,
  extra: Partial<GroupBreakdownMonth> = {},
): GroupBreakdownMonth {
  return {
    year_month: `${ym}-01T00:00:00Z`,
    total,
    by_subtype: bySubtype,
    missing_fx: [],
    ...extra,
  };
}

function breakdown(group: GroupBreakdown["group"], months: GroupBreakdownMonth[]): GroupBreakdown {
  return { group, reporting_currency: "IDR", months };
}

const assetItem = (id: string, currency: string, amount: string, subtype = "bank_account") => ({
  asset: { id, native_currency: currency, status: "active", terminated_at: null, subtype },
  latest_snapshot: { amount },
});

function stubAssets(opts: {
  multi: boolean;
  items: Record<string, ReturnType<typeof assetItem>[]>;
  breakdown?: GroupBreakdown;
}) {
  server.use(
    http.get("/api/me", () => HttpResponse.json(me(opts.multi))),
    http.get("/api/bank-accounts", () => HttpResponse.json(opts.items.bank ?? [])),
    http.get("/api/properties", () => HttpResponse.json(opts.items.property ?? [])),
    http.get("/api/vehicles", () => HttpResponse.json(opts.items.vehicle ?? [])),
    http.get("/api/assets/time-series", () => HttpResponse.json([])),
    ...(opts.breakdown
      ? [http.get("/api/reports/groups/asset", () => HttpResponse.json(opts.breakdown))]
      : []),
  );
  return renderWithProviders(
    <MemoryRouter>
      <AssetsHome />
    </MemoryRouter>,
  );
}

const TWO_CATEGORIES = [
  month("2026-02", "2500000", { bank_account: "1000000", property: "1500000" }),
  month("2026-03", "3100000", { bank_account: "1100000", property: "2000000" }),
];

describe("AssetsHome with a foreign currency held (multi-currency on)", () => {
  const items = {
    bank: [assetItem("a1", "IDR", "1100000"), assetItem("a2", "USD", "100")],
    property: [assetItem("a3", "IDR", "2000000", "property")],
  };

  it("renders one chart set in the reporting currency, headlined by the backend's converted total", async () => {
    stubAssets({ multi: true, items, breakdown: breakdown("asset", TWO_CATEGORIES) });

    // The headline is the converted total as the backend sent it — not a client
    // sum of the native lists (which would be 3,100,100 mixing IDR and USD).
    const total = await screen.findByTestId("home-total-value");
    expect(flat(total.textContent)).toContain(money("3100000", "IDR"));

    expect(await screen.findByTestId("home-value-IDR")).toBeInTheDocument();
    expect(screen.getByTestId("home-category-stack-IDR")).toBeInTheDocument();
    expect(screen.getByTestId("home-category-pie-IDR")).toBeInTheDocument();
    // One set, not one per native currency.
    expect(screen.queryByTestId("home-value-USD")).not.toBeInTheDocument();
    expect(screen.queryByTestId("home-category-pie-USD")).not.toBeInTheDocument();
    expect(screen.queryByTestId("missing-fx-warning")).not.toBeInTheDocument();
  });

  it("shows the native subtotal of the foreign currency as a muted line", async () => {
    stubAssets({ multi: true, items, breakdown: breakdown("asset", TWO_CATEGORIES) });
    const line = await screen.findByTestId("home-total-native");
    expect(flat(line.textContent)).toContain(`incl. ${money("100", "USD")}`);
    // The reporting currency is the headline itself, not repeated in the line.
    expect(flat(line.textContent)).not.toContain(money("3100000", "IDR"));
    expect(line.textContent).toContain("converted at each month's rate");
  });

  it("shows the native line even when the foreign currency is the only one held", async () => {
    stubAssets({
      multi: true,
      items: { bank: [assetItem("a2", "USD", "100")] },
      breakdown: breakdown("asset", [month("2026-03", "1600000", { bank_account: "1600000" })]),
    });
    const line = await screen.findByTestId("home-total-native");
    expect(flat(line.textContent)).toContain(`incl. ${money("100", "USD")}`);
  });

  it("warns, with the dashboard's own line, when a month's USD position has no rate", async () => {
    const months = [
      TWO_CATEGORIES[0],
      month(
        "2026-03",
        "3000000",
        { bank_account: "1000000", property: "2000000" },
        { missing_fx: [{ position_id: "a2", currency: "USD" }] },
      ),
    ];
    stubAssets({ multi: true, items, breakdown: breakdown("asset", months) });

    const warning = await screen.findByTestId("missing-fx-warning");
    expect(warning).toHaveTextContent(/This total excludes 1 item/);
    expect(warning).toHaveTextContent(/exchange rate for USD/);
    // The headline is the figure that excludes it.
    expect(flat(screen.getByTestId("home-total-value").textContent)).toContain(
      money("3000000", "IDR"),
    );
  });

  it("draws no composition chart when only one category is held", async () => {
    const months = [
      month("2026-02", "1000000", { bank_account: "1000000" }),
      month("2026-03", "1100000", { bank_account: "1100000" }),
    ];
    stubAssets({ multi: true, items, breakdown: breakdown("asset", months) });

    expect(await screen.findByTestId("home-value-IDR")).toBeInTheDocument();
    expect(screen.queryByTestId("home-category-stack-IDR")).not.toBeInTheDocument();
    expect(screen.queryByTestId("home-category-pie-IDR")).not.toBeInTheDocument();
  });
});

describe("AssetsHome without conversion", () => {
  it("keeps one card set per native currency, and never asks for the converted breakdown", async () => {
    // Multi-currency off: no /api/reports/groups handler is registered, so a
    // request for it would fail the test (onUnhandledFrame: "error").
    stubAssets({
      multi: false,
      items: {
        bank: [assetItem("a1", "IDR", "100"), assetItem("a2", "USD", "5")],
        property: [
          assetItem("a3", "IDR", "900", "property"),
          assetItem("a4", "USD", "7", "property"),
        ],
      },
    });
    // Each currency is summed on its own (IDR 100 + 900, USD 5 + 7) — never mixed.
    const headline = await screen.findByTestId("home-total");
    expect(flat(headline.textContent)).toContain(money("1000", "IDR"));
    expect(flat(headline.textContent)).toContain(money("12", "USD"));
    expect(screen.queryByTestId("home-total-native")).not.toBeInTheDocument();
    expect(screen.queryByTestId("missing-fx-warning")).not.toBeInTheDocument();
  });

  it("does not convert a multi-currency household that holds only the reporting currency", async () => {
    stubAssets({
      multi: true,
      items: { bank: [assetItem("a1", "IDR", "100")] },
    });
    expect(await screen.findByTestId("home-total")).toBeInTheDocument();
    expect(screen.queryByTestId("home-total-value")).not.toBeInTheDocument();
  });
});

describe("LiabilitiesHome with a foreign currency held", () => {
  it("renders the converted total, its native subtotal, and hides a single-category pie", async () => {
    const liab = (id: string, currency: string, amount: string) => ({
      liability: {
        id,
        native_currency: currency,
        status: "active",
        terminated_at: null,
        subtype: "personal",
      },
      latest_snapshot: { amount },
    });
    server.use(
      http.get("/api/me", () => HttpResponse.json(me(true))),
      http.get("/api/liabilities", () =>
        HttpResponse.json([liab("l1", "IDR", "500"), liab("l2", "USD", "20")]),
      ),
      http.get("/api/liabilities/time-series", () => HttpResponse.json([])),
      http.get("/api/reports/groups/liability", () =>
        HttpResponse.json(
          breakdown("liability", [
            month("2026-02", "800", { personal: "800" }),
            month("2026-03", "800", { personal: "800" }),
          ]),
        ),
      ),
    );
    renderWithProviders(
      <MemoryRouter>
        <LiabilitiesHome />
      </MemoryRouter>,
    );
    expect(flat((await screen.findByTestId("home-total-value")).textContent)).toContain(
      money("800", "IDR"),
    );
    expect(flat(screen.getByTestId("home-total-native").textContent)).toContain(money("20", "USD"));
    expect(await screen.findByTestId("home-value-IDR")).toBeInTheDocument();
    expect(screen.queryByTestId("home-category-stack-IDR")).not.toBeInTheDocument();
    expect(screen.queryByTestId("home-category-pie-IDR")).not.toBeInTheDocument();
  });
});

describe("InvestmentsHome with a foreign currency held", () => {
  it("renders one converted set; the risk pie is hidden when one risk profile holds everything", async () => {
    const inv = (id: string, currency: string, amount: string, risk: string) => ({
      investment: {
        id,
        native_currency: currency,
        status: "active",
        terminated_at: null,
        risk_profile: risk,
      },
      latest_snapshot: { amount },
      cost_basis: "0",
    });
    const rm = (ym: string, total: string, by: Record<string, string>) =>
      month(ym, total, by, { by_risk: { low: "0", medium: total, high: "0" } });
    server.use(
      http.get("/api/me", () => HttpResponse.json(me(true))),
      http.get("/api/investments/stocks", () =>
        HttpResponse.json([inv("s1", "IDR", "100", "medium"), inv("s2", "USD", "10", "medium")]),
      ),
      http.get("/api/investments/mutual-funds", () => HttpResponse.json([])),
      http.get("/api/investments/bonds", () => HttpResponse.json([])),
      http.get("/api/investments/time-deposits", () => HttpResponse.json([])),
      http.get("/api/investments/golds", () => HttpResponse.json([])),
      http.get("/api/investments/time-series", () => HttpResponse.json([])),
      http.get("/api/reports/groups/investment", () =>
        HttpResponse.json(
          breakdown("investment", [
            rm("2026-02", "1000", { stock: "600", mutual_fund: "400" }),
            rm("2026-03", "1200", { stock: "700", mutual_fund: "500" }),
          ]),
        ),
      ),
    );
    renderWithProviders(
      <MemoryRouter>
        <InvestmentsHome />
      </MemoryRouter>,
    );
    expect(flat((await screen.findByTestId("home-total-value")).textContent)).toContain(
      money("1200", "IDR"),
    );
    expect(flat(screen.getByTestId("home-total-native").textContent)).toContain(money("10", "USD"));
    expect(await screen.findByTestId("home-value-cost-IDR")).toBeInTheDocument();
    expect(screen.queryByTestId("home-value-cost-USD")).not.toBeInTheDocument();
    expect(screen.getByTestId("home-category-stack-IDR")).toBeInTheDocument();
    expect(screen.getByTestId("home-category-pie-IDR")).toBeInTheDocument();
    expect(screen.queryByTestId("home-risk-pie-IDR")).not.toBeInTheDocument();
  });
});

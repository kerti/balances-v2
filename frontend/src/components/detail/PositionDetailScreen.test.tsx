// The generic detail screen's own chrome (#839): the Snapshots | Transactions tabs
// and the header's ⋯ menu. Driven through real descriptors over MSW — a stock (has
// a ledger) and a bank account (has none) — so the tab bar's presence is decided
// by the position's shape, not by a prop.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useLocation } from "react-router";
import { toast } from "sonner";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { raisedToasts } from "@/test/sonnerMock";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { stockDescriptor } from "@/components/detail/descriptors/stock";
import { bankAccountDescriptor } from "@/components/detail/descriptors/bankAccount";
import type {
  Asset,
  AssetSnapshot,
  Investment,
  InvestmentSnapshot,
  InvestmentTransaction,
} from "@/api/types";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const audit = {
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
};

const investment: Investment = {
  id: "i1",
  household_id: "h1",
  display_name: "Acme Corp Shares",
  description: null,
  subtype: "stock",
  ownership_type: "joint",
  entry_type: "acquired" as const,
  sole_owner_user_id: null,
  native_currency: "USD",
  status: "active",
  terminated_at: null,
  termination_note: null,
  risk_profile: "medium",
  rolled_from_investment_id: null,
  tag_id: null,
  ...audit,
};

const asset: Asset = {
  id: "a1",
  household_id: "h1",
  display_name: "Everyday Checking",
  description: null,
  subtype: "bank_account",
  ownership_type: "joint",
  entry_type: "acquired" as const,
  sole_owner_user_id: null,
  native_currency: "USD",
  tag_id: null,
  status: "active",
  terminated_at: null,
  termination_note: null,
  ...audit,
};

function stockSnapshot(id: string, ym: string): InvestmentSnapshot {
  return {
    id,
    investment_id: "i1",
    year_month: `${ym}-01T00:00:00Z`,
    amount: "4000.00",
    currency: "USD",
    quantity: "40",
    price_per_unit: "100.00",
    accrued_interest: null,
    as_of_date: null,
    description: null,
    ...audit,
  };
}

function trade(id: string): InvestmentTransaction {
  return {
    id,
    investment_id: "i1",
    transaction_type: "buy",
    transaction_date: "2026-05-01T00:00:00Z",
    currency: "USD",
    description: null,
    amount: "1000.00",
    quantity: "10",
    price_per_unit: "100.00",
    principal_amount: null,
    interest_amount: null,
    principal_disposition: null,
    interest_disposition: null,
    ...audit,
  };
}

function stubStock(status = "active") {
  server.use(
    http.get("/api/investments/stocks/i1", () =>
      HttpResponse.json({
        investment: {
          ...investment,
          status,
          terminated_at: status === "active" ? null : "2026-10-01T00:00:00Z",
        },
        details: { investment_id: "i1", ticker: "ACME", exchange: "NYSE" },
      }),
    ),
    http.get("/api/investments/i1/snapshots", () =>
      HttpResponse.json([
        stockSnapshot("s3", "2026-06"),
        stockSnapshot("s2", "2026-05"),
        stockSnapshot("s1", "2026-04"),
      ]),
    ),
    http.get("/api/investments/i1/transactions", () =>
      HttpResponse.json([trade("t1"), trade("t2")]),
    ),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.get("/api/me", () => HttpResponse.json({ id: "u1", locale: "en-GB", theme: "system" })),
  );
}

function stubBankAccount() {
  const snapshot: AssetSnapshot = {
    id: "s1",
    asset_id: "a1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "4321.00",
    currency: "USD",
    as_of_date: null,
    description: null,
    ...audit,
  };
  server.use(
    http.get("/api/bank-accounts/a1", () =>
      HttpResponse.json({
        asset,
        details: {
          asset_id: "a1",
          bank_name: "Test Bank",
          account_number: "1234",
          account_type: "savings",
        },
      }),
    ),
    http.get("/api/assets/a1/snapshots", () => HttpResponse.json([snapshot])),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.get("/api/me", () => HttpResponse.json({ id: "u1", locale: "en-GB", theme: "system" })),
  );
}

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{`${location.pathname}${location.search}`}</div>;
}

function renderStock(route = "/investments/stocks/i1", onBack = vi.fn()) {
  renderWithProviders(
    <>
      <PositionDetailScreen descriptor={stockDescriptor} assetId="i1" onBack={onBack} />
      <LocationProbe />
    </>,
    { route },
  );
  return onBack;
}

beforeEach(() => vi.clearAllMocks());

// covers: INV-PRESENTATION-27
describe("history tabs", () => {
  it("a position with a ledger shows Snapshots (n) | Transactions (n), Snapshots first", async () => {
    stubStock();
    renderStock();

    const snapshotsTab = await screen.findByRole("tab", { name: "Snapshots (3)" });
    const transactionsTab = screen.getByRole("tab", { name: "Transactions (2)" });
    expect(snapshotsTab).toHaveAttribute("aria-selected", "true");
    expect(transactionsTab).toHaveAttribute("aria-selected", "false");
    expect(screen.getByTestId("history-panel-snapshots")).toHaveAttribute("data-state", "active");
    expect(screen.getByTestId("history-panel-transactions")).toHaveAttribute(
      "data-state",
      "inactive",
    );
  });

  it("a position without a ledger shows the snapshots list and no tab bar", async () => {
    stubBankAccount();
    renderWithProviders(
      <PositionDetailScreen descriptor={bankAccountDescriptor} assetId="a1" onBack={vi.fn()} />,
      { route: "/assets/bank-accounts/a1" },
    );

    await screen.findByTestId("tour-snapshots");
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(screen.queryByTestId("history-tabs")).not.toBeInTheDocument();
  });

  it("?tab=transactions deep-links to the Transactions tab", async () => {
    stubStock();
    renderStock("/investments/stocks/i1?tab=transactions");

    const transactionsTab = await screen.findByRole("tab", { name: "Transactions (2)" });
    expect(transactionsTab).toHaveAttribute("aria-selected", "true");
    expect(screen.getByTestId("history-panel-transactions")).toHaveAttribute(
      "data-state",
      "active",
    );
    expect(screen.getByTestId("history-panel-snapshots")).toHaveAttribute("data-state", "inactive");
  });

  it("an unknown ?tab= value falls back to Snapshots", async () => {
    stubStock();
    renderStock("/investments/stocks/i1?tab=nonsense");

    const snapshotsTab = await screen.findByRole("tab", { name: "Snapshots (3)" });
    expect(snapshotsTab).toHaveAttribute("aria-selected", "true");
  });

  it("?tab=transactions on a position with no ledger is ignored", async () => {
    stubBankAccount();
    renderWithProviders(
      <PositionDetailScreen descriptor={bankAccountDescriptor} assetId="a1" onBack={vi.fn()} />,
      { route: "/assets/bank-accounts/a1?tab=transactions" },
    );

    expect(await screen.findByTestId("tour-snapshots")).toBeVisible();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  });

  it("switching tabs writes ?tab= into the URL, and back to Snapshots clears it", async () => {
    stubStock();
    renderStock();
    const user = userEvent.setup();

    await user.click(await screen.findByRole("tab", { name: "Transactions (2)" }));
    expect(screen.getByTestId("location")).toHaveTextContent(
      "/investments/stocks/i1?tab=transactions",
    );
    expect(screen.getByTestId("history-panel-transactions")).toHaveAttribute(
      "data-state",
      "active",
    );

    await user.click(screen.getByRole("tab", { name: "Snapshots (3)" }));
    expect(screen.getByTestId("location")).toHaveTextContent(/^\/investments\/stocks\/i1$/);
  });

  it("the Transactions tab count stays the ledger size while the search narrows the list", async () => {
    stubStock();
    renderStock("/investments/stocks/i1?tab=transactions");
    const user = userEvent.setup();

    await screen.findByRole("tab", { name: "Transactions (2)" });
    await user.type(screen.getByTestId("txn-search"), "no such transaction");

    await waitFor(() =>
      expect(
        within(screen.getByTestId("tour-transactions")).queryAllByTestId("transaction-row"),
      ).toHaveLength(0),
    );
    expect(screen.getByRole("tab", { name: "Transactions (2)" })).toBeInTheDocument();
  });
});

describe("header ⋯ menu", () => {
  async function openMenu() {
    const user = userEvent.setup();
    const actions = await screen.findByTestId("tour-actions");
    await user.click(within(actions).getByTestId("position-menu-trigger"));
    return user;
  }

  it("the header is Help, Edit, ⋯ — and Close… / Delete are not loose buttons", async () => {
    stubStock();
    renderStock();

    const actions = await screen.findByTestId("tour-actions");
    const buttons = within(actions)
      .getAllByRole("button")
      .map((b) => b.getAttribute("data-testid") ?? b.textContent);
    expect(buttons).toEqual(["help-tour", "Edit", "position-menu-trigger"]);
  });

  it("an open position offers Close… then, after a separator, a red Delete", async () => {
    stubStock();
    renderStock();
    await openMenu();

    const menu = await screen.findByRole("menu");
    const items = within(menu).getAllByRole("menuitem");
    expect(items.map((i) => i.textContent)).toEqual(["Close…", "Delete"]);
    expect(within(menu).getByRole("separator")).toBeInTheDocument();
    expect(items[1]).toHaveAttribute("data-variant", "destructive");
    expect(within(menu).queryByRole("menuitem", { name: "Change status" })).toBeNull();
  });

  it("a closed position offers Change status instead of Close…", async () => {
    stubStock("sold");
    renderStock();
    await openMenu();

    const menu = await screen.findByRole("menu");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((i) => i.textContent),
    ).toEqual(["Change status", "Delete"]);
  });

  it("Close… opens the status dialog", async () => {
    stubStock();
    renderStock();
    const user = await openMenu();

    await user.click(await screen.findByRole("menuitem", { name: "Close…" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/status/i)).toBeInTheDocument();
  });

  // covers: INV-PRESENTATION-16
  it("Delete asks first in the red confirm, then deletes with an Undo toast", async () => {
    stubStock();
    let deleted = false;
    server.use(
      http.delete("/api/investments/stocks/i1", () => {
        deleted = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const onBack = renderStock();
    const user = await openMenu();

    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const confirm = await screen.findByRole("alertdialog");
    expect(within(confirm).getByText("Delete Acme Corp Shares?")).toBeInTheDocument();
    expect(deleted).toBe(false);

    const confirmButton = within(confirm).getByRole("button", { name: "Delete" });
    expect(confirmButton).toHaveAttribute("data-variant", "destructive");
    await user.click(confirmButton);

    await waitFor(() => expect(onBack).toHaveBeenCalled());
    expect(deleted).toBe(true);
    expect(raisedToasts(mockedToast).some((t) => t.options.action?.label === "Undo")).toBe(true);
  });
});

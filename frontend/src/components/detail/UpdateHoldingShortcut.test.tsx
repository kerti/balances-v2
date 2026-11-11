// "Update {Mon} holding →" (#830): after a Buy/Sell the trade's toast, and the
// quantity-mismatch banner, offer to open Record snapshot prefilled with the
// ledger quantity and the latest price. Driven through the real stock descriptor
// over stateful MSW endpoints. The load-bearing property (INV-SNAPSHOTS-18): the
// action only opens the dialog - no snapshot is written until the user saves it.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { raisedToasts } from "@/test/sonnerMock";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { stockDescriptor } from "@/components/detail/descriptors/stock";
import type { Investment, InvestmentSnapshot, InvestmentTransaction } from "@/api/types";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

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
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
  risk_profile: "medium",
  rolled_from_investment_id: null,
  tag_id: null,
};

const audit = {
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
};

function snapshot(id: string, ym: string, quantity: string, price: string): InvestmentSnapshot {
  return {
    id,
    investment_id: "i1",
    year_month: `${ym}-01T00:00:00Z`,
    amount: String(Number(quantity) * Number(price)),
    currency: "USD",
    quantity,
    price_per_unit: price,
    accrued_interest: null,
    as_of_date: null,
    description: null,
    ...audit,
  };
}

function ledgerRow(id: string, over: Partial<InvestmentTransaction>): InvestmentTransaction {
  return {
    id,
    investment_id: "i1",
    transaction_type: "buy",
    transaction_date: "2026-05-01T00:00:00Z",
    currency: "USD",
    description: null,
    amount: null,
    quantity: null,
    price_per_unit: null,
    principal_amount: null,
    interest_amount: null,
    principal_disposition: null,
    interest_disposition: null,
    ...audit,
    ...over,
  };
}

// The 40 sh holding: one Buy of 40 and a June snapshot of 40 at 100, so the
// ledger and the snapshot agree until the test records another trade.
let snapshots: InvestmentSnapshot[];
let ledger: InvestmentTransaction[];
let snapshotWrites: unknown[];

beforeEach(() => {
  vi.clearAllMocks();
  snapshots = [snapshot("s1", "2026-06", "40", "100.00")];
  ledger = [ledgerRow("t1", { quantity: "40", price_per_unit: "100.00", amount: "4000.00" })];
  snapshotWrites = [];
  server.use(
    http.get("/api/investments/stocks/i1", () =>
      HttpResponse.json({
        investment,
        details: { investment_id: "i1", ticker: "ACME", exchange: "NYSE" },
      }),
    ),
    http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/investments/i1/transactions", () => HttpResponse.json(ledger)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.get("/api/me", () => HttpResponse.json({ id: "u1", locale: "en-GB", theme: "system" })),
    http.post("/api/investments/i1/transactions", async ({ request }) => {
      const body = (await request.json()) as Partial<InvestmentTransaction>;
      const row = ledgerRow("t2", { ...body, transaction_date: "2026-11-10T00:00:00Z" });
      ledger = [row, ...ledger];
      return HttpResponse.json(row, { status: 201 });
    }),
    http.post("/api/investments/i1/snapshots", async ({ request }) => {
      const body = (await request.json()) as Record<string, string>;
      snapshotWrites.push(body);
      const created = snapshot("s2", body.year_month, body.quantity, body.price_per_unit);
      snapshots = [created, ...snapshots];
      return HttpResponse.json(created, { status: 201 });
    }),
  );
});

async function renderStock() {
  renderWithProviders(
    <PositionDetailScreen descriptor={stockDescriptor} assetId="i1" onBack={vi.fn()} />,
  );
  await screen.findByTestId("tour-transactions");
}

async function recordBuy(quantity: string, price: string) {
  await userEvent.click(screen.getByRole("button", { name: "Buy" }));
  await userEvent.type(screen.getByLabelText(/Quantity \(sh\)/), quantity);
  await userEvent.type(screen.getByLabelText(/Price per unit/), price);
  await userEvent.click(screen.getByRole("button", { name: "Record buy" }));
}

const holdingAction = () =>
  raisedToasts(mockedToast).find((t) => t.options.action?.label.includes("holding"));

// covers: INV-SNAPSHOTS-18
describe("Update holding shortcut", () => {
  it("a Buy of 10 on 40 offers the action on its one toast, which only prefills Record snapshot", async () => {
    await renderStock();
    await recordBuy("10", "120");

    // Exactly one toast for the write; it says Added, offers the shortcut, and
    // carries no Undo (a create has none - ADR-0058).
    await waitFor(() => expect(raisedToasts(mockedToast)).toHaveLength(1));
    const [added] = raisedToasts(mockedToast);
    expect(added.message).toBe("Added");
    expect(added.options.action?.label).toBe("Update Jul holding →");
    expect(added.options.action?.label).not.toMatch(/undo/i);

    // The trade itself wrote no snapshot, and showing the toast writes none.
    expect(snapshotWrites).toEqual([]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    // Clicking it opens Record snapshot on July with the ledger quantity (40 + 10)
    // and the latest snapshot's price (100, not this trade's 120) - still no write.
    act(() => added.options.action!.onClick());
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Quantity")).toHaveValue("50");
    expect(within(dialog).getByLabelText(/Price per unit/)).toHaveValue("100.00");
    expect(within(dialog).getByText(/Jul 2026|Jul/)).toBeInTheDocument();
    expect(snapshotWrites).toEqual([]);

    // Only Save writes, with the confirmed values; the mismatch warning then clears.
    expect(await screen.findByTestId("reconcile-banner")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Save snapshot" }));
    await waitFor(() => expect(snapshotWrites).toHaveLength(1));
    expect(snapshotWrites[0]).toMatchObject({
      year_month: "2026-07",
      quantity: "50",
      price_per_unit: "100.00",
      amount: "5000",
    });
    await waitFor(() => expect(screen.queryByTestId("reconcile-banner")).not.toBeInTheDocument());
  });

  it("lets the user edit the prefilled values before saving", async () => {
    await renderStock();
    await recordBuy("10", "120");
    await waitFor(() => expect(holdingAction()).toBeDefined());
    act(() => holdingAction()!.options.action!.onClick());

    const dialog = await screen.findByRole("dialog");
    const price = within(dialog).getByLabelText(/Price per unit/);
    await userEvent.clear(price);
    await userEvent.type(price, "125");
    await userEvent.click(within(dialog).getByRole("button", { name: "Save snapshot" }));

    await waitFor(() => expect(snapshotWrites).toHaveLength(1));
    expect(snapshotWrites[0]).toMatchObject({ quantity: "50", price_per_unit: "125" });
  });

  it("a Sell offers the same action, prefilled with the reduced ledger quantity", async () => {
    await renderStock();
    await userEvent.click(screen.getByRole("button", { name: "Sell" }));
    await userEvent.type(screen.getByLabelText(/Quantity \(sh\)/), "15");
    await userEvent.type(screen.getByLabelText(/Price per unit/), "130");
    await userEvent.click(screen.getByRole("button", { name: "Record sell" }));

    await waitFor(() => expect(holdingAction()).toBeDefined());
    act(() => holdingAction()!.options.action!.onClick());
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Quantity")).toHaveValue("25");
    expect(snapshotWrites).toEqual([]);
  });

  it("a non-trade write (a fee) keeps the plain Added toast with no action", async () => {
    await renderStock();
    await userEvent.click(screen.getByRole("button", { name: /^Fee/ }));
    await userEvent.type(screen.getByLabelText(/Cash amount/), "5");
    await userEvent.click(screen.getByRole("button", { name: /Record fee/ }));

    await waitFor(() => expect(raisedToasts(mockedToast)).toHaveLength(1));
    expect(raisedToasts(mockedToast)[0].options.action).toBeUndefined();
  });

  it("the mismatch banner carries the same action", async () => {
    // The ledger says 50 while the latest snapshot still says 40.
    ledger = [
      ledgerRow("t0", { quantity: "10", price_per_unit: "100.00", amount: "1000.00" }),
      ...ledger,
    ];
    await renderStock();

    const banner = await screen.findByTestId("reconcile-banner");
    expect(banner).toHaveTextContent(/doesn't match ledger total/);
    expect(snapshotWrites).toEqual([]);

    const action = within(banner).getByTestId("reconcile-update-holding");
    expect(action).toHaveTextContent("Update Jul holding →");
    await userEvent.click(action);

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Quantity")).toHaveValue("50");
    expect(within(dialog).getByLabelText(/Price per unit/)).toHaveValue("100.00");
    // Opening it wrote nothing.
    expect(snapshotWrites).toEqual([]);

    await userEvent.click(within(dialog).getByRole("button", { name: "Save snapshot" }));
    await waitFor(() => expect(snapshotWrites).toHaveLength(1));
    await waitFor(() => expect(screen.queryByTestId("reconcile-banner")).not.toBeInTheDocument());
  });

  it("the banner offers no action on a closed position (Record snapshot is hidden there)", async () => {
    investmentOverrideStatus("sold");
    ledger = [
      ledgerRow("t0", { quantity: "10", price_per_unit: "100.00", amount: "1000.00" }),
      ...ledger,
    ];
    await renderStock();

    const banner = await screen.findByTestId("reconcile-banner");
    expect(within(banner).queryByTestId("reconcile-update-holding")).not.toBeInTheDocument();
  });
});

function investmentOverrideStatus(status: string) {
  server.use(
    http.get("/api/investments/stocks/i1", () =>
      HttpResponse.json({
        investment: { ...investment, status, terminated_at: "2026-10-01T00:00:00Z" },
        details: { investment_id: "i1", ticker: "ACME", exchange: "NYSE" },
      }),
    ),
  );
}

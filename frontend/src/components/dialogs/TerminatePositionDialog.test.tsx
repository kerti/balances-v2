// Component test for the Investment settlement capture in the terminate dialog
// (ADR-0052 §6, #587). The dialog is one generic surface for all four position
// groups; what is asserted here is the Investment-only half — that closing a
// position captures the Transaction saying where its value went, in the shape
// the subtype's own transaction matrix accepts, and sends it on the SAME request
// as the flip.
//
// The atomicity that request buys is a backend guarantee (proved in
// repo/investment_settlement_test.go); what this tier can prove is that the
// frontend never splits it into two calls.
import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { TerminatePositionDialog } from "@/components/dialogs/TerminatePositionDialog";
import type { InvestmentSubtype } from "@/lib/lifecycle";

const ID = "inv-1";

// One January snapshot: 100 units marked at 9,500 (the qty×price shape), or a
// deposit carrying 4,500,000 of accrued interest inside its 104,500,000 total
// (the accrued shape). The dialog seeds its defaults from whichever applies.
const qtyPriceSnapshots = [
  {
    id: "s1",
    year_month: "2026-01",
    amount: "950000",
    quantity: "100",
    price_per_unit: "9500",
    accrued_interest: null,
  },
];

const accruedSnapshots = [
  {
    id: "s1",
    year_month: "2026-01",
    amount: "104500000",
    quantity: null,
    price_per_unit: null,
    accrued_interest: "4500000",
  },
];

const buyLedger = [
  {
    id: "t1",
    investment_id: ID,
    transaction_type: "buy",
    transaction_date: "2026-01-05",
    amount: "950000",
    quantity: "100",
    price_per_unit: "9500",
  },
];

type Captured = { body: unknown };

// Stubs the three endpoints the dialog touches and returns the captured PATCH
// body, so a test can assert the settlement rode along with the flip.
function stubInvestment(snapshots: unknown[], transactions: unknown[], captured: Captured): void {
  server.use(
    http.get(`/api/investments/${ID}/snapshots`, () => HttpResponse.json(snapshots)),
    http.get(`/api/investments/${ID}/transactions`, () => HttpResponse.json(transactions)),
    http.patch(`/api/investments/${ID}/lifecycle`, async ({ request }) => {
      captured.body = await request.json();
      return HttpResponse.json({ id: ID, status: "sold" });
    }),
  );
}

function renderDialog(subtype: InvestmentSubtype | undefined, currentStatus = "active") {
  return renderWithProviders(
    <TerminatePositionDialog
      group={subtype ? "investments" : "assets"}
      id={ID}
      listKey="stocks"
      currentStatus={currentStatus}
      currentTerminatedAt={null}
      currentNote={null}
      investmentSubtype={subtype}
      currency="IDR"
    />,
  );
}

async function openDialog(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("terminate-position-trigger"));
  return screen.findByRole("dialog");
}

describe("TerminatePositionDialog settlement capture", () => {
  // covers: INV-LIFECYCLE-08
  it("sends the sale on the same request as the flip, sized from the ledger", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    stubInvestment(qtyPriceSnapshots, buyLedger, captured);
    renderDialog("stock");

    await openDialog(user);
    // Picking the terminal status reveals the settlement block, pre-filled from
    // the held quantity and the last marked price.
    await user.selectOptions(screen.getByLabelText(/status/i), "sold");

    const block = await screen.findByTestId("terminate-settlement");
    await waitFor(() => expect(screen.getByTestId("settlement-quantity")).toHaveValue("100"));
    expect(screen.getByTestId("settlement-price")).toHaveValue("9500");
    expect(within(block).getByTestId("settlement-total")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(captured.body).toBeDefined());
    expect(captured.body).toMatchObject({
      status: "sold",
      settlement: {
        quantity: "100",
        price_per_unit: "9500",
        principal_amount: null,
        interest_amount: null,
      },
    });
  });

  // covers: INV-LIFECYCLE-08
  it("the write-off escape sends a zero-priced sale, not an absent one", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    stubInvestment(qtyPriceSnapshots, buyLedger, captured);
    renderDialog("stock");

    await openDialog(user);
    await user.selectOptions(screen.getByLabelText(/status/i), "sold");
    await screen.findByTestId("terminate-settlement");

    await user.click(screen.getByTestId("settlement-write-off"));
    expect(screen.getByTestId("settlement-price")).toBeDisabled();

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(captured.body).toBeDefined());
    // The quantity still leaves the position — only the price is zero. A total
    // loss is a truthful negative Investment Return (ADR-0052 §5), and the Sell
    // is what records that its fate is known.
    expect(captured.body).toMatchObject({
      settlement: { quantity: "100", price_per_unit: "0" },
    });
  });

  // covers: INV-LIFECYCLE-08
  it("captures principal and interest for a deposit, whose matrix has no Sell", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    stubInvestment(accruedSnapshots, [], captured);
    renderDialog("time_deposit");

    await openDialog(user);
    // A TimeDeposit offers no `sold` — its only settleable terminal status is
    // `matured`, because Maturity is the only Transaction it accepts. It still
    // offers `untracked`, which settles nothing and so sits outside the matrix.
    const statusSelect = screen.getByLabelText(/status/i) as HTMLSelectElement;
    expect([...statusSelect.options].map((o) => o.value)).toEqual([
      "active",
      "matured",
      "untracked",
    ]);

    await user.selectOptions(statusSelect, "matured");
    await screen.findByTestId("terminate-settlement");

    await waitFor(() =>
      expect(screen.getByTestId("settlement-principal")).toHaveValue("100000000"),
    );
    expect(screen.getByTestId("settlement-interest")).toHaveValue("4500000");

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(captured.body).toBeDefined());
    expect(captured.body).toMatchObject({
      status: "matured",
      settlement: {
        quantity: null,
        price_per_unit: null,
        principal_amount: "100000000",
        interest_amount: "4500000",
      },
    });
  });

  // covers: INV-LIFECYCLE-08
  it("stays submittable for a position that was never marked or funded", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    // No snapshots, no ledger — a position created and closed without ever being
    // marked. There is nothing to price, so the settlement is 0 × 0 and the form
    // must still submit; a required-but-blank price silently blocked the close.
    stubInvestment([], [], captured);
    renderDialog("stock");

    await openDialog(user);
    await user.selectOptions(screen.getByLabelText(/status/i), "sold");
    await screen.findByTestId("terminate-settlement");

    await waitFor(() => expect(screen.getByTestId("settlement-price")).toHaveValue("0"));
    expect(screen.getByTestId("settlement-quantity")).toHaveValue("0");

    await user.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(captured.body).toBeDefined());
    expect(captured.body).toMatchObject({ status: "sold" });
  });

  // covers: INV-LIFECYCLE-08
  it("leaves the price blank when the position holds something but was never marked", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    // Bought but never marked. Defaulting the price to 0 here would book a real
    // sale as a total loss, so it stays blank and the user must state it.
    stubInvestment([], buyLedger, captured);
    renderDialog("stock");

    await openDialog(user);
    await user.selectOptions(screen.getByLabelText(/status/i), "sold");
    await screen.findByTestId("terminate-settlement");

    await waitFor(() => expect(screen.getByTestId("settlement-quantity")).toHaveValue("100"));
    expect(screen.getByTestId("settlement-price")).toHaveValue("");
    expect(screen.getByTestId("settlement-price")).toBeRequired();
  });

  // covers: INV-LIFECYCLE-08
  it("offers no settlement when the termination month already carries a sale", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    const sold = [
      ...buyLedger,
      {
        id: "t2",
        investment_id: ID,
        transaction_type: "sell",
        transaction_date: "2026-03-10",
        amount: "1100000",
        quantity: "100",
        price_per_unit: "11000",
      },
    ];
    stubInvestment(qtyPriceSnapshots, sold, captured);
    renderDialog("stock");

    await openDialog(user);
    await user.selectOptions(screen.getByLabelText(/status/i), "sold");
    await user.clear(screen.getByLabelText(/terminated on/i));
    await user.type(screen.getByLabelText(/terminated on/i), "2026-03-10");

    await screen.findByTestId("terminate-already-settled");
    expect(screen.queryByTestId("terminate-settlement")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(captured.body).toBeDefined());
    expect(captured.body).not.toHaveProperty("settlement");
  });

  // covers: INV-LIFECYCLE-08
  it("leaves the other three groups untouched — no settlement, full status list", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    server.use(
      http.patch(`/api/assets/${ID}/lifecycle`, async ({ request }) => {
        captured.body = await request.json();
        return HttpResponse.json({ id: ID, status: "sold" });
      }),
    );
    renderDialog(undefined);

    await openDialog(user);
    const statusSelect = screen.getByLabelText(/status/i) as HTMLSelectElement;
    expect([...statusSelect.options].map((o) => o.value)).toEqual([
      "active",
      "closed",
      "sold",
      "disposed",
      "untracked",
    ]);

    await user.selectOptions(statusSelect, "sold");
    expect(screen.queryByTestId("terminate-settlement")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(captured.body).toBeDefined());
    expect(captured.body).not.toHaveProperty("settlement");
  });
});

// The exit side of a Tracking Change (#595, ADR-0053 §5). `untracked` is the
// one terminal status every group offers, and the one exempt from the
// settlement capture above — a departing member's holdings were not sold, so
// there are no proceeds to record and nothing to write off.
describe("TerminatePositionDialog untracked termination", () => {
  // covers: INV-LIFECYCLE-09
  it("offers untracked to an Investment and captures no settlement for it", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    // A TimeDeposit: the narrowest matrix there is (Maturity only), so if
    // untracked survives here it survives everywhere.
    stubInvestment(accruedSnapshots, [], captured);
    renderDialog("time_deposit");

    await openDialog(user);
    const statusSelect = screen.getByLabelText(/status/i) as HTMLSelectElement;
    expect([...statusSelect.options].map((o) => o.value)).toEqual([
      "active",
      "matured",
      "untracked",
    ]);

    await user.selectOptions(statusSelect, "untracked");
    // No settlement block, and no write-off escape either — both assert a cash
    // leg that did not happen.
    expect(screen.queryByTestId("terminate-settlement")).not.toBeInTheDocument();
    expect(screen.queryByTestId("settlement-write-off")).not.toBeInTheDocument();
    // The distinguishing copy is what stops it being read as "sold".
    expect(screen.getByTestId("terminate-untracked-hint")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(captured.body).toBeDefined());
    expect(captured.body).toMatchObject({ status: "untracked" });
    expect(captured.body).not.toHaveProperty("settlement");
    // The biconditional still holds: a terminal status carries a date.
    expect((captured.body as { terminated_at: string }).terminated_at).toMatch(
      /^\d{4}-\d{2}-\d{2}$/,
    );
  });

  // covers: INV-LIFECYCLE-09
  it("offers untracked to the other three groups too", async () => {
    const user = userEvent.setup();
    renderDialog(undefined);

    await openDialog(user);
    const statusSelect = screen.getByLabelText(/status/i) as HTMLSelectElement;
    expect([...statusSelect.options].map((o) => o.value)).toContain("untracked");

    await user.selectOptions(statusSelect, "untracked");
    expect(screen.getByTestId("terminate-untracked-hint")).toBeInTheDocument();
  });

  // covers: INV-LIFECYCLE-09
  it("drops the hint again when the status moves off untracked", async () => {
    const user = userEvent.setup();
    const captured: Captured = { body: undefined };
    stubInvestment(qtyPriceSnapshots, buyLedger, captured);
    renderDialog("stock");

    await openDialog(user);
    const statusSelect = screen.getByLabelText(/status/i) as HTMLSelectElement;
    await user.selectOptions(statusSelect, "untracked");
    expect(screen.getByTestId("terminate-untracked-hint")).toBeInTheDocument();

    // Switching to a settled status brings the capture back with its defaults
    // intact — the untracked detour must not have poisoned them.
    await user.selectOptions(statusSelect, "sold");
    expect(screen.queryByTestId("terminate-untracked-hint")).not.toBeInTheDocument();
    await screen.findByTestId("terminate-settlement");
    await waitFor(() => expect(screen.getByTestId("settlement-quantity")).toHaveValue("100"));
    expect(screen.getByTestId("settlement-price")).toHaveValue("9500");
  });
});

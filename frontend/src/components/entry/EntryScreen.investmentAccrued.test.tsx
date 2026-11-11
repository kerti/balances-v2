// Conformance test for the Investment accrued bulk monthly-entry view
// (ADR-0046, #424). The accrued twin of the qty×price component test: drives the
// shared EntryScreen with the accrued config over MSW-stubbed accrued endpoints
// and asserts the accrued contract — carry-forward prefill seeds total value +
// accrued interest with the principal computed, the per-row accrued default
// follows coupon disposition (accrues → forced entry, pays_out/time-deposit → 0),
// Save sends only the changed rows (dirty-only) keyed by investment_id with the
// amount + accrued_interest columns (no quantity/price), and a 422 marks the row.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { investmentAccruedEntryConfig } from "@/components/entry/groups";
import { thisYearMonth } from "@/lib/dateLimits";

const entryList = {
  year_month: "2026-05",
  rows: [
    {
      investment_id: "b1",
      display_name: "FR Series Bond",
      currency: "IDR",
      subtype: "bond",
      ownership_type: "joint",
      sole_owner_user_id: null,
      coupon_disposition: "pays_out",
      prefill_amount: "50250000",
      prefill_accrued_interest: "250000",
      carried_from: "2026-04",
    },
    {
      investment_id: "b2",
      display_name: "Accrues Bond",
      currency: "IDR",
      subtype: "bond",
      ownership_type: "joint",
      sole_owner_user_id: null,
      coupon_disposition: "accrues",
      prefill_amount: null,
      prefill_accrued_interest: null,
      carried_from: null,
    },
    {
      investment_id: "t1",
      display_name: "BCA 12-month",
      currency: "IDR",
      subtype: "time_deposit",
      ownership_type: "joint",
      sole_owner_user_id: null,
      coupon_disposition: null,
      principal: "100000000",
      prefill_amount: null,
      prefill_accrued_interest: null,
      carried_from: null,
    },
  ],
};

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <EntryScreen config={investmentAccruedEntryConfig} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
// covers: INV-SNAPSHOTS-09
it("renders eligible accrued positions grouped by subtype with prefill + disposition default", async () => {
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();

  // The bond with history carries its last snapshot's total value + accrued...
  const amount = await screen.findByTestId("investment-accrued-entry-amount-b1");
  expect(amount).toHaveValue("50,250,000");
  expect(screen.getByTestId("investment-accrued-entry-accrued-b1")).toHaveValue("250,000");
  // ...and the principal is computed (50,250,000 − 250,000 = 50,000,000).
  expect(screen.getByTestId("investment-accrued-entry-value-b1")).toHaveTextContent(/50/);

  // A fresh accrues bond forces a real accrued entry — the field starts empty.
  expect(screen.getByTestId("investment-accrued-entry-amount-b2")).toHaveValue("");
  expect(screen.getByTestId("investment-accrued-entry-accrued-b2")).toHaveValue("");

  // A time deposit takes one Value and no accrued field (#844): its interest is
  // derived, so there is nothing to default.
  expect(screen.getByTestId("investment-accrued-entry-amount-t1")).toHaveValue("");
  expect(screen.queryByTestId("investment-accrued-entry-accrued-t1")).not.toBeInTheDocument();

  // Grouped by subtype: a Bonds section (b1, b2) and a Time deposits section (t1).
  const bonds = screen.getByTestId("investment-accrued-entry-group-bond");
  expect(within(bonds).getByTestId("investment-accrued-entry-row-b1")).toBeInTheDocument();
  expect(within(bonds).getByTestId("investment-accrued-entry-row-b2")).toBeInTheDocument();
  const tds = screen.getByTestId("investment-accrued-entry-group-time_deposit");
  expect(within(tds).getByTestId("investment-accrued-entry-row-t1")).toBeInTheDocument();

  // Every column is named once per group, including the derived one ("Principal"
  // for accrued), so the computed figure isn't the row's one unlabelled column.
  expect(within(bonds).getByText("Total value")).toBeInTheDocument();
  expect(within(bonds).getByText("Accrued")).toBeInTheDocument();
  expect(within(bonds).getByText("Principal")).toBeInTheDocument();
  // The time-deposit group is headed by its own single Value column and the
  // derived interest — not the bond's total/accrued/principal trio.
  expect(within(tds).getByText("Value")).toBeInTheDocument();
  expect(within(tds).getByText("Interest")).toBeInTheDocument();
  expect(within(tds).queryByText("Accrued")).not.toBeInTheDocument();
});

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-09
it("Save sends only changed rows keyed by investment_id with amount + accrued_interest", async () => {
  let posted: {
    rows: Array<{
      investment_id: string;
      amount: string;
      accrued_interest: string;
      currency: string;
    }>;
  } | null = null;
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/accrued/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen();
  const user = userEvent.setup();

  // Fill in the fresh accrues bond's two figures; leave the others at prefill.
  const b2Amount = await screen.findByTestId("investment-accrued-entry-amount-b2");
  await user.type(b2Amount, "10100000");
  await user.type(screen.getByTestId("investment-accrued-entry-accrued-b2"), "100000");
  await user.click(screen.getByTestId("investment-accrued-entry-save"));

  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({
    investment_id: "b2",
    amount: "10100000",
    accrued_interest: "100000",
    currency: "IDR",
  });
  // The accrued branch carries no quantity/price.
  expect(posted!.rows[0]).not.toHaveProperty("quantity");
  expect(posted!.rows[0]).not.toHaveProperty("price_per_unit");
});

// covers: INV-SNAPSHOTS-09
it("a fresh accrues bond with only the total value typed is not dirty (accrued forced)", async () => {
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();
  const user = userEvent.setup();

  // Type only the total value for the accrues bond — its accrued default is
  // empty (forced entry), so the row is incomplete and not saveable.
  const b2Amount = await screen.findByTestId("investment-accrued-entry-amount-b2");
  await user.type(b2Amount, "10100000");

  expect(screen.getByTestId("investment-accrued-entry-dirty-count")).toHaveTextContent(/0/);
  expect(screen.getByTestId("investment-accrued-entry-save")).toBeDisabled();
});

// covers: INV-SNAPSHOTS-06
it("marks the offending row on a 422 per-row error", async () => {
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/accrued/bulk", () =>
      HttpResponse.json({ errors: [{ investment_id: "b1", code: "ineligible" }] }, { status: 422 }),
    ),
  );
  renderScreen();
  const user = userEvent.setup();

  const accrued = await screen.findByTestId("investment-accrued-entry-accrued-b1");
  await user.clear(accrued);
  await user.type(accrued, "300000");
  await user.click(screen.getByTestId("investment-accrued-entry-save"));

  expect(await screen.findByTestId("investment-accrued-entry-error-b1")).toBeInTheDocument();
});

// covers: INV-SNAPSHOTS-09
// covers: INV-SNAPSHOTS-17
it("a time deposit row derives its interest live, warns below the principal, and saves only the Value", async () => {
  let posted: { rows: Array<Record<string, unknown>> } | null = null;
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/accrued/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1, replaced: 0 });
    }),
  );
  renderScreen();
  const user = userEvent.setup();

  const value = await screen.findByTestId("investment-accrued-entry-amount-t1");
  const interest = screen.getByTestId("investment-accrued-entry-value-t1");
  // Nothing typed: no interest yet, no warning.
  expect(interest).toHaveTextContent("—");
  expect(screen.queryByTestId("investment-accrued-entry-warning-t1")).not.toBeInTheDocument();

  // 101,500,000 against a 100,000,000 principal: 1,500,000 of interest.
  await user.type(value, "101500000");
  expect(interest).toHaveTextContent("1,500,000");
  expect(screen.queryByTestId("investment-accrued-entry-warning-t1")).not.toBeInTheDocument();

  // Below the principal: a soft warning, and the row still saves.
  await user.clear(value);
  await user.type(value, "99000000");
  expect(interest).toHaveTextContent("-1,000,000");
  expect(screen.getByTestId("investment-accrued-entry-warning-t1")).toHaveTextContent(
    "Value is below the principal",
  );
  expect(screen.getByTestId("investment-accrued-entry-save")).toBeEnabled();

  await user.click(screen.getByTestId("investment-accrued-entry-save"));
  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  // One figure goes up: the server derives accrued_interest, so none is sent.
  expect(posted!.rows[0]).toEqual({ investment_id: "t1", currency: "IDR", amount: "99000000" });
});

// covers: INV-SNAPSHOTS-09
it("a time deposit with a snapshot this month shows its Value, not its interest, as Current", async () => {
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () =>
      HttpResponse.json({
        year_month: "2026-05",
        rows: [
          {
            ...entryList.rows[2],
            prefill_amount: "101500000",
            prefill_accrued_interest: "1500000",
            carried_from: thisYearMonth(),
          },
        ],
      }),
    ),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();

  const current = await screen.findByTestId("investment-accrued-entry-current-t1");
  expect(current).toHaveTextContent("101,500,000");
});

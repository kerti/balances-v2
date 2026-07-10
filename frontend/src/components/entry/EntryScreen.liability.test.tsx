// Conformance test for the Liability bulk monthly-entry view (ADR-0046, #422).
// The grouped-group twin of the Asset tracer's component test: drives the shared
// EntryScreen with the Liability config over MSW-stubbed liability endpoints and
// asserts the same user-facing contract — carry-forward prefill renders grouped
// by subtype, Save sends only the rows the user changed (dirty-only) keyed by
// liability_id, and a 422 marks the offending row.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { liabilityEntryConfig } from "@/components/entry/groups";

const entryList = {
  year_month: "2026-05",
  rows: [
    {
      liability_id: "l1",
      display_name: "Home loan",
      currency: "IDR",
      subtype: "institutional",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_amount: "500000000",
      carried_from: "2026-04",
    },
    {
      liability_id: "p1",
      display_name: "Loan from Mum",
      currency: "IDR",
      subtype: "personal",
      ownership_type: "sole",
      sole_owner_user_id: "u1",
      prefill_amount: null,
      carried_from: null,
    },
  ],
};

const members = [
  { id: "u1", display_name: "Pat Owner", nickname: null, email: "pat@example.test" },
];

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <EntryScreen config={liabilityEntryConfig} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
it("renders eligible liabilities grouped by subtype with carry-forward prefill", async () => {
  server.use(
    http.get("/api/liabilities/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json(members)),
  );
  renderScreen();

  const withHist = await screen.findByTestId("liability-entry-amount-l1");
  expect(withHist).toHaveValue("500000000");
  expect(screen.getByTestId("liability-entry-amount-p1")).toHaveValue("");
  expect(screen.getByTestId("liability-entry-row-p1")).toHaveTextContent(/no previous value/i);

  // Grouped by subtype: a Personal section (p1) and an Institutional section
  // (l1), each labelled from liabilities:home.categoryLabel.*.
  const personal = screen.getByTestId("liability-entry-group-personal");
  expect(personal).toHaveTextContent(/personal/i);
  expect(within(personal).getByTestId("liability-entry-row-p1")).toBeInTheDocument();
  const institutional = screen.getByTestId("liability-entry-group-institutional");
  expect(institutional).toHaveTextContent(/institutional/i);
  expect(within(institutional).getByTestId("liability-entry-row-l1")).toBeInTheDocument();

  await waitFor(() =>
    expect(screen.getByTestId("liability-entry-row-p1")).toHaveTextContent(/pat owner/i),
  );
});

// covers: INV-SNAPSHOTS-06
it("Save sends only the rows the user changed, keyed by liability_id", async () => {
  let posted: { rows: Array<{ liability_id: string; amount: string; currency: string }> } | null =
    null;
  server.use(
    http.get("/api/liabilities/snapshots/entry", () => HttpResponse.json(entryList)),
    http.post("/api/liabilities/snapshots/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen();
  const user = userEvent.setup();

  const fresh = await screen.findByTestId("liability-entry-amount-p1");
  await user.type(fresh, "3000000");
  await user.click(screen.getByTestId("liability-entry-save"));

  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({ liability_id: "p1", amount: "3000000", currency: "IDR" });
});

// covers: INV-SNAPSHOTS-06
it("marks the offending row on a 422 per-row error", async () => {
  server.use(
    http.get("/api/liabilities/snapshots/entry", () => HttpResponse.json(entryList)),
    http.post("/api/liabilities/snapshots/bulk", () =>
      HttpResponse.json({ errors: [{ liability_id: "l1", code: "ineligible" }] }, { status: 422 }),
    ),
  );
  renderScreen();
  const user = userEvent.setup();

  const withHist = await screen.findByTestId("liability-entry-amount-l1");
  await user.clear(withHist);
  await user.type(withHist, "400000000");
  await user.click(screen.getByTestId("liability-entry-save"));

  expect(await screen.findByTestId("liability-entry-error-l1")).toBeInTheDocument();
});

// Conformance test for the Receivable bulk monthly-entry view (ADR-0046, #422).
// The flat-group twin of the Asset tracer's component test: receivables have no
// subtype, so the shared EntryScreen renders one ungrouped list (no
// receivable-entry-group-* sections). Drives the Receivable config over
// MSW-stubbed receivable endpoints and asserts the same contract — carry-forward
// prefill renders, Save sends only the rows the user changed (dirty-only) keyed
// by receivable_id, and a 422 marks the offending row.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { receivableEntryConfig } from "@/components/entry/groups";

const entryList = {
  year_month: "2026-05",
  rows: [
    {
      receivable_id: "r1",
      display_name: "Owed by Sam",
      currency: "IDR",
      subtype: "",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_amount: "2000000",
      carried_from: "2026-04",
    },
    {
      receivable_id: "r2",
      display_name: "New IOU",
      currency: "IDR",
      subtype: "",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_amount: null,
      carried_from: null,
    },
  ],
};

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <EntryScreen config={receivableEntryConfig} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
it("renders eligible receivables as one flat list with carry-forward prefill", async () => {
  server.use(
    http.get("/api/receivables/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();

  const withHist = await screen.findByTestId("receivable-entry-amount-r1");
  expect(withHist).toHaveValue("2000000");
  expect(screen.getByTestId("receivable-entry-amount-r2")).toHaveValue("");
  expect(screen.getByTestId("receivable-entry-row-r2")).toHaveTextContent(/no previous value/i);

  // Flat group: no subtype sections are rendered.
  expect(screen.queryByTestId("receivable-entry-group-")).not.toBeInTheDocument();
});

// covers: INV-SNAPSHOTS-06
it("Save sends only the rows the user changed, keyed by receivable_id", async () => {
  let posted: { rows: Array<{ receivable_id: string; amount: string; currency: string }> } | null =
    null;
  server.use(
    http.get("/api/receivables/snapshots/entry", () => HttpResponse.json(entryList)),
    http.post("/api/receivables/snapshots/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen();
  const user = userEvent.setup();

  const fresh = await screen.findByTestId("receivable-entry-amount-r2");
  await user.type(fresh, "750000");
  await user.click(screen.getByTestId("receivable-entry-save"));

  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({ receivable_id: "r2", amount: "750000", currency: "IDR" });
});

// covers: INV-SNAPSHOTS-06
it("marks the offending row on a 422 per-row error", async () => {
  server.use(
    http.get("/api/receivables/snapshots/entry", () => HttpResponse.json(entryList)),
    http.post("/api/receivables/snapshots/bulk", () =>
      HttpResponse.json({ errors: [{ receivable_id: "r1", code: "ineligible" }] }, { status: 422 }),
    ),
  );
  renderScreen();
  const user = userEvent.setup();

  const withHist = await screen.findByTestId("receivable-entry-amount-r1");
  await user.clear(withHist);
  await user.type(withHist, "1000000");
  await user.click(screen.getByTestId("receivable-entry-save"));

  expect(await screen.findByTestId("receivable-entry-error-r1")).toBeInTheDocument();
});

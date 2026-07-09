// Component test for the Asset bulk monthly-entry view (ADR-0046, #421). Drives
// the real screen over MSW-stubbed entry + bulk endpoints and asserts the
// user-facing contract: carry-forward prefill renders, Save sends only the rows
// the user changed (dirty-only), and a 422 marks the offending row.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { AssetEntryScreen } from "@/components/AssetEntryScreen";

const entryList = {
  year_month: "2026-05",
  rows: [
    {
      asset_id: "a1",
      display_name: "Main checking",
      currency: "IDR",
      prefill_amount: "1000000",
      carried_from: "2026-04",
    },
    {
      asset_id: "a2",
      display_name: "New account",
      currency: "IDR",
      prefill_amount: null,
      carried_from: null,
    },
  ],
};

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <AssetEntryScreen />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
it("renders eligible assets with their carry-forward prefill", async () => {
  server.use(http.get("/api/assets/snapshots/entry", () => HttpResponse.json(entryList)));
  renderScreen();

  const withHist = await screen.findByTestId("asset-entry-amount-a1");
  expect(withHist).toHaveValue("1000000");
  expect(screen.getByTestId("asset-entry-amount-a2")).toHaveValue("");
  expect(screen.getByTestId("asset-entry-row-a2")).toHaveTextContent(/no previous value/i);
});

// covers: INV-SNAPSHOTS-06
it("Save sends only the rows the user changed", async () => {
  let posted: { rows: Array<{ asset_id: string; amount: string }> } | null = null;
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(entryList)),
    http.post("/api/assets/snapshots/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen();
  const user = userEvent.setup();

  // Touch only the fresh account; leave the carried-forward one untouched.
  const fresh = await screen.findByTestId("asset-entry-amount-a2");
  await user.type(fresh, "500000");
  await user.click(screen.getByTestId("asset-entry-save"));

  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({ asset_id: "a2", amount: "500000", currency: "IDR" });
});

// covers: INV-SNAPSHOTS-06
it("marks the offending row on a 422 per-row error", async () => {
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(entryList)),
    http.post("/api/assets/snapshots/bulk", () =>
      HttpResponse.json({ errors: [{ asset_id: "a1", code: "ineligible" }] }, { status: 422 }),
    ),
  );
  renderScreen();
  const user = userEvent.setup();

  const withHist = await screen.findByTestId("asset-entry-amount-a1");
  await user.clear(withHist);
  await user.type(withHist, "2000000");
  await user.click(screen.getByTestId("asset-entry-save"));

  expect(await screen.findByTestId("asset-entry-error-a1")).toBeInTheDocument();
});

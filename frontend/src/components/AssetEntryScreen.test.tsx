// Component test for the Asset bulk monthly-entry view (ADR-0046, #421). Drives
// the real screen over MSW-stubbed entry + bulk endpoints and asserts the
// user-facing contract: carry-forward prefill renders, Save sends only the rows
// the user changed (dirty-only), and a 422 marks the offending row.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { thisYearMonth } from "@/lib/dateLimits";
import { AssetEntryScreen } from "@/components/AssetEntryScreen";

const entryList = {
  year_month: "2026-05",
  rows: [
    {
      asset_id: "a1",
      display_name: "Main checking",
      currency: "IDR",
      subtype: "bank_account",
      ownership_type: "sole",
      sole_owner_user_id: "u1",
      prefill_amount: "1000000",
      carried_from: "2026-04",
    },
    {
      asset_id: "a2",
      display_name: "New account",
      currency: "IDR",
      subtype: "bank_account",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_amount: null,
      carried_from: null,
    },
    {
      asset_id: "v1",
      display_name: "Family car",
      currency: "IDR",
      subtype: "vehicle",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_amount: "80000000",
      carried_from: "2026-04",
    },
  ],
};

const members = [
  { id: "u1", display_name: "Pat Owner", nickname: null, email: "pat@example.test" },
];

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <AssetEntryScreen />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
it("renders eligible assets grouped by type, with owner + carry-forward prefill", async () => {
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json(members)),
  );
  renderScreen();

  const withHist = await screen.findByTestId("asset-entry-amount-a1");
  expect(withHist).toHaveValue("1000000");
  expect(screen.getByTestId("asset-entry-amount-a2")).toHaveValue("");
  expect(screen.getByTestId("asset-entry-row-a2")).toHaveTextContent(/no previous value/i);

  // Rows are grouped by type: a Bank accounts section (a1, a2) and a Vehicles
  // section (v1), each labelled.
  const banks = screen.getByTestId("asset-entry-group-bank_account");
  expect(banks).toHaveTextContent(/bank accounts/i);
  expect(within(banks).getByTestId("asset-entry-row-a1")).toBeInTheDocument();
  expect(within(banks).getByTestId("asset-entry-row-a2")).toBeInTheDocument();
  const vehicles = screen.getByTestId("asset-entry-group-vehicle");
  expect(vehicles).toHaveTextContent(/vehicles/i);
  expect(within(vehicles).getByTestId("asset-entry-row-v1")).toBeInTheDocument();

  // Owner shows per row: a1 is sole-owned by Pat, a2 is joint.
  await waitFor(() =>
    expect(screen.getByTestId("asset-entry-row-a1")).toHaveTextContent(/pat owner/i),
  );
  expect(screen.getByTestId("asset-entry-row-a2")).toHaveTextContent(/joint/i);

  // Back button is present for consistency with the detail pages.
  expect(screen.getByTestId("asset-entry-back")).toBeInTheDocument();
});

// covers: INV-SNAPSHOTS-06
it("warns that editing overwrites when a snapshot already exists for the month", async () => {
  const ym = thisYearMonth(); // the screen's default target month
  server.use(
    http.get("/api/assets/snapshots/entry", () =>
      HttpResponse.json({
        year_month: ym,
        rows: [
          {
            asset_id: "x1",
            display_name: "Already entered",
            currency: "IDR",
            subtype: "bank_account",
            ownership_type: "joint",
            sole_owner_user_id: null,
            prefill_amount: "999",
            carried_from: ym, // a snapshot already exists for this month
          },
        ],
      }),
    ),
    http.get("/api/household/members", () => HttpResponse.json(members)),
  );
  renderScreen();

  expect(await screen.findByTestId("asset-entry-overwrite-x1")).toBeInTheDocument();
  // A genuinely carried-forward row does not carry the overwrite warning.
  expect(screen.queryByText(/carried from/i)).not.toBeInTheDocument();
});

it("marks a changed row and Undo reverts it to the prefill", async () => {
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json(members)),
  );
  renderScreen();
  const user = userEvent.setup();

  const input = await screen.findByTestId("asset-entry-amount-a1"); // prefilled 1000000
  const undo = screen.getByTestId("asset-entry-undo-a1");
  expect(undo).toBeDisabled();
  expect(screen.queryByTestId("asset-entry-dirty-a1")).not.toBeInTheDocument();

  await user.clear(input);
  await user.type(input, "2500000");
  expect(screen.getByTestId("asset-entry-dirty-a1")).toBeInTheDocument();
  expect(undo).toBeEnabled();

  await user.click(undo);
  expect(input).toHaveValue("1000000");
  expect(screen.queryByTestId("asset-entry-dirty-a1")).not.toBeInTheDocument();
  expect(screen.getByTestId("asset-entry-undo-a1")).toBeDisabled();
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

// The statement date is never pre-filled (ADR-0046 amendment, #790): bulk entry
// starts it blank, saving without touching it sends null (the row stores
// as_of_date = NULL), a date the user picks is sent as picked, and changing the
// month drops a date picked for the old one rather than re-seeding it.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { assetEntryConfig } from "@/components/entry/groups";

type Posted = { year_month: string; as_of_date: string | null };

const entryList = {
  year_month: "2026-03",
  rows: [
    {
      asset_id: "a1",
      display_name: "New account",
      currency: "IDR",
      subtype: "bank_account",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_amount: null,
      carried_from: null,
    },
  ],
};

function setup() {
  const captured: { body: Posted | null } = { body: null };
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/assets/snapshots/bulk", async ({ request }) => {
      captured.body = (await request.json()) as Posted;
      return HttpResponse.json({ written: 1, replaced: 0 });
    }),
  );
  renderWithProviders(
    <MemoryRouter initialEntries={["/assets/enter?month=2026-03"]}>
      <EntryScreen config={assetEntryConfig} />
    </MemoryRouter>,
  );
  return captured;
}

// covers: INV-SNAPSHOTS-13
it("saves with no statement date when the user never touched it", async () => {
  const captured = setup();
  const user = userEvent.setup();

  const date = await screen.findByTestId("asset-entry-asof");
  expect(date).toHaveValue("");
  await user.type(await screen.findByTestId("asset-entry-amount-a1"), "500000");
  await user.click(screen.getByTestId("asset-entry-save"));

  await waitFor(() => expect(captured.body).not.toBeNull());
  expect(captured.body!.year_month).toBe("2026-03");
  expect(captured.body!.as_of_date).toBeNull();
});

// covers: INV-SNAPSHOTS-13
it("sends a statement date the user picked", async () => {
  const captured = setup();
  const user = userEvent.setup();

  await user.type(await screen.findByTestId("asset-entry-amount-a1"), "500000");
  await user.type(screen.getByTestId("asset-entry-asof"), "2026-03-14");
  await user.click(screen.getByTestId("asset-entry-save"));

  await waitFor(() => expect(captured.body).not.toBeNull());
  expect(captured.body!.as_of_date).toBe("2026-03-14");
});

// covers: INV-SNAPSHOTS-13
it("clears a picked date on a month change instead of re-seeding one", async () => {
  setup();
  const user = userEvent.setup();

  const date = await screen.findByTestId("asset-entry-asof");
  await user.type(date, "2026-03-14");
  expect(date).toHaveValue("2026-03-14");

  await user.click(screen.getByTestId("month-picker-prev"));
  await waitFor(() => expect(screen.getByTestId("asset-entry-asof")).toHaveValue(""));
});

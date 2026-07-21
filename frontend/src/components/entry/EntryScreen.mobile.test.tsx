// Mobile-renderer conformance for the amount-only bulk monthly-entry view
// (ADR-0050 S1, #502). Flips window.innerWidth below the 768px boundary so
// useIsMobile() picks EntryRowMobile, then asserts the divergence contract: the
// SAME data-testids resolve as on desktop, the value input meets the ≥44px tap
// floor (h-11), and all behaviour (dirty-only atomic Save keyed by position)
// still lives in the shared EntryScreen container — no logic forks into the
// renderer. Deep per-shape assertions stay in the desktop conformance twins.
import { it, expect, afterEach } from "vitest";
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

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <EntryScreen config={receivableEntryConfig} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
it("mobile: renders the stacked amount input under the same testid and meets the 44px tap floor", async () => {
  setViewport(500);
  server.use(
    http.get("/api/receivables/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();

  const input = await screen.findByTestId("receivable-entry-amount-r1");
  // Same testid contract as desktop, carry-forward prefill intact.
  expect(input).toHaveValue("2000000");
  // A11y floor: the value input is a full-width ≥44px (h-11) target — the mobile
  // renderer, not the desktop w-36 row.
  expect(input).toHaveClass("h-11");
  expect(input).not.toHaveClass("w-36");
  expect(screen.getByTestId("receivable-entry-row-r2")).toHaveTextContent(/no previous value/i);
});

// covers: INV-SNAPSHOTS-06
it("mobile: Save still sends only the dirty rows from the shared container", async () => {
  setViewport(500);
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
  // Dirty accounting is the container's, unchanged by the renderer swap.
  expect(screen.getByTestId("receivable-entry-dirty-count")).toHaveTextContent("1 changed");
  await user.click(screen.getByTestId("receivable-entry-save"));

  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({ receivable_id: "r2", amount: "750000", currency: "IDR" });
});

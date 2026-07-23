// Mobile-renderer conformance for the accrued bulk monthly-entry view
// (ADR-0050 S3, #506). The accrued twin of the amount-only (#502) and qty×price
// (#505) mobile tests: flips window.innerWidth below the 768px boundary so
// useIsMobile() picks EntryRowMobile, then asserts the divergence contract for
// the accrued shape — total value and accrued interest each stack on their own
// full-width ≥44px line (not the cramped desktop w-28 columns), the derived
// principal shows in the footer named "Principal" (mobile has no desktop column
// header to label it, so the footer carries the name — distinguishing it from a
// qty×price "Value"), and the container-only per-row accrued default (accrues →
// forced entry, pays-out / time deposit → 0) survives the renderer swap
// untouched. The SAME data-testids resolve as on desktop, so the deep per-shape
// assertions stay in the desktop twin (EntryScreen.investmentAccrued.test.tsx).
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { investmentAccruedEntryConfig } from "@/components/entry/groups";

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
      prefill_amount: null,
      prefill_accrued_interest: null,
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
      <EntryScreen config={investmentAccruedEntryConfig} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
// covers: INV-SNAPSHOTS-09
it("mobile: stacks total value + accrued as full-width ≥44px lines with the principal named", async () => {
  setViewport(500);
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();

  // Same testid contract as desktop, carry-forward prefill intact on both tab-stops.
  const amount = await screen.findByTestId("investment-accrued-entry-amount-b1");
  const accrued = screen.getByTestId("investment-accrued-entry-accrued-b1");
  expect(amount).toHaveValue("50250000");
  expect(accrued).toHaveValue("250000");
  // A11y floor: each tab-stop is a full-width ≥44px (h-11) target — the mobile
  // renderer, not the cramped desktop w-28 columns.
  expect(amount).toHaveClass("h-11");
  expect(amount).not.toHaveClass("w-28");
  expect(accrued).toHaveClass("h-11");
  expect(accrued).not.toHaveClass("w-28");
  // Mobile labels each field inline within its own row (desktop labels them once
  // per group header instead); the inline <Label>s wire to the same inputs.
  const bondRow = screen.getByTestId("investment-accrued-entry-row-b1");
  expect(within(bondRow).getByText("Total value")).toBeInTheDocument();
  expect(within(bondRow).getByText("Accrued")).toBeInTheDocument();
  expect(within(bondRow).getByLabelText("Total value")).toBe(amount);
  expect(within(bondRow).getByLabelText("Accrued")).toBe(accrued);
  // The derived principal (50,250,000 − 250,000 = 50,000,000) shows in the footer
  // named "Principal" — not the qty×price "Value" — so a mobile user reads it as
  // the of-which-principal breakdown, not another field to fill.
  const principal = screen.getByTestId("investment-accrued-entry-value-b1");
  expect(principal).toHaveTextContent("Principal");
  expect(principal).toHaveTextContent(/50/);

  // The per-row accrued default follows coupon disposition, unchanged by the
  // renderer swap: an accrues bond forces a real entry (empty), a time deposit
  // (no disposition ⇒ pays-out) defaults to 0.
  expect(screen.getByTestId("investment-accrued-entry-accrued-b2")).toHaveValue("");
  expect(screen.getByTestId("investment-accrued-entry-accrued-t1")).toHaveValue("0");
});

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-09
it("mobile: a fresh accrues bond with only the total typed stays not-dirty — the container rule survives the swap", async () => {
  setViewport(500);
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();
  const user = userEvent.setup();

  // Type only the total value for the fresh accrues bond — its accrued default is
  // empty (forced entry, #66), so the row is incomplete. The "accrued forced ⇒
  // not yet saveable" rule lives in EntryScreen, not the renderer, so the mobile
  // swap must not change it.
  const b2Amount = await screen.findByTestId("investment-accrued-entry-amount-b2");
  await user.type(b2Amount, "10100000");

  expect(screen.getByTestId("investment-accrued-entry-dirty-count")).toHaveTextContent(/0/);
  expect(screen.getByTestId("investment-accrued-entry-save")).toBeDisabled();
  // Principal stays a placeholder until both figures are present.
  expect(screen.getByTestId("investment-accrued-entry-value-b2")).toHaveTextContent("—");
});

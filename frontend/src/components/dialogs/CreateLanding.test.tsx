// The create flow (#843): every position Create dialog opens on Sole owner
// (you) and, on success, hands the new item's id to the list so it can land on
// the detail page; the detail route then opens Record snapshot exactly once.
import { describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useNavigate, useLocation } from "react-router";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { DetailRoute } from "@/components/shell/DetailRoute";
import { CreateBankAccountDialog } from "@/components/dialogs/CreateBankAccountDialog";
import { CreateBondDialog } from "@/components/dialogs/CreateBondDialog";
import { CreateGoldDialog } from "@/components/dialogs/CreateGoldDialog";
import { CreateLiabilityDialog } from "@/components/dialogs/CreateLiabilityDialog";
import { CreateMutualFundDialog } from "@/components/dialogs/CreateMutualFundDialog";
import { CreatePropertyDialog } from "@/components/dialogs/CreatePropertyDialog";
import { CreateReceivableDialog } from "@/components/dialogs/CreateReceivableDialog";
import { CreateStockDialog } from "@/components/dialogs/CreateStockDialog";
import { CreateTimeDepositDialog } from "@/components/dialogs/CreateTimeDepositDialog";
import { CreateVehicleDialog } from "@/components/dialogs/CreateVehicleDialog";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { arrivalOptions } from "@/lib/recordOnArrival";

const me = {
  id: "u1",
  household_id: "h1",
  household_display_name: "Test Household",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
  picture_url: null,
  locale: "en-GB",
  theme: "system",
  time_zone: "UTC",
  reporting_currency: "IDR",
  multi_currency_enabled: false,
  assumed_annual_inflation: "3.5",
  is_founder: true,
};

function stubSession() {
  server.use(
    http.get("/api/me", () => HttpResponse.json(me)),
    http.get("/api/household/members", () =>
      HttpResponse.json([
        { id: "u1", display_name: "Pat Owner", nickname: null, email: "pat@example.test" },
        { id: "u2", display_name: "Sam Partner", nickname: null, email: "sam@example.test" },
      ]),
    ),
  );
}

const dialogs: [string, () => React.ReactElement][] = [
  ["bank account", () => <CreateBankAccountDialog />],
  ["property", () => <CreatePropertyDialog />],
  ["vehicle", () => <CreateVehicleDialog />],
  ["liability", () => <CreateLiabilityDialog />],
  ["receivable", () => <CreateReceivableDialog />],
  ["stock", () => <CreateStockDialog />],
  ["mutual fund", () => <CreateMutualFundDialog />],
  ["bond", () => <CreateBondDialog />],
  ["gold", () => <CreateGoldDialog />],
  ["time deposit", () => <CreateTimeDepositDialog />],
];

describe("Create dialogs default ownership", () => {
  // covers: INV-PRESENTATION-20
  it.each(dialogs)("%s opens on Sole owner (you)", async (_name, make) => {
    stubSession();
    renderWithProviders(make());
    await userEvent.click(screen.getByRole("button"));

    expect(await screen.findByRole("radio", { name: "Sole owner" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Joint" })).not.toBeChecked();
    // The current user is the selected owner.
    const owner = await screen.findByRole("combobox", { name: "Sole owner" });
    await waitFor(() => expect(owner).toHaveValue("u1"));
  });
});

describe("Create dialogs hand the new id to the list", () => {
  // covers: INV-PRESENTATION-20
  it("asset: a bank account saves without an account number and reports its id", async () => {
    stubSession();
    let body: Record<string, unknown> = {};
    server.use(
      http.post("/api/bank-accounts", async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ asset: { id: "new-asset" }, details: {} }, { status: 201 });
      }),
    );
    const onCreated = vi.fn();
    renderWithProviders(<CreateBankAccountDialog onCreated={onCreated} />);
    await userEvent.click(screen.getByRole("button"));
    await userEvent.type(await screen.findByLabelText("Display name"), "Everyday");
    await userEvent.type(screen.getByLabelText("Bank name"), "Test Bank");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith("new-asset"));
    expect(body.account_number).toBe("");
    expect(body.ownership_type).toBe("sole");
    expect(body.sole_owner_user_id).toBe("u1");
  });

  // covers: INV-PRESENTATION-20
  it("liability: reports the new liability's id", async () => {
    stubSession();
    server.use(
      http.post("/api/liabilities", () => HttpResponse.json({ id: "new-liab" }, { status: 201 })),
    );
    const onCreated = vi.fn();
    renderWithProviders(<CreateLiabilityDialog onCreated={onCreated} />);
    await userEvent.click(screen.getByRole("button"));
    await userEvent.type(await screen.findByLabelText("Display name"), "Car loan");
    await userEvent.type(screen.getByLabelText("Counterparty"), "Test Lender");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith("new-liab"));
  });

  // covers: INV-PRESENTATION-20
  it("receivable: reports the new receivable's id", async () => {
    stubSession();
    server.use(
      http.post("/api/receivables", () => HttpResponse.json({ id: "new-rec" }, { status: 201 })),
    );
    const onCreated = vi.fn();
    renderWithProviders(<CreateReceivableDialog onCreated={onCreated} />);
    await userEvent.click(screen.getByRole("button"));
    await userEvent.type(await screen.findByLabelText("Display name"), "Friend loan");
    await userEvent.type(screen.getByLabelText("Counterparty"), "Test Friend");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith("new-rec"));
  });

  // covers: INV-PRESENTATION-20
  it("investment: reports the new gold item's id", async () => {
    stubSession();
    server.use(
      http.post("/api/investments/golds", () =>
        HttpResponse.json({ investment: { id: "new-gold" }, details: {} }, { status: 201 }),
      ),
    );
    const onCreated = vi.fn();
    renderWithProviders(<CreateGoldDialog onCreated={onCreated} />);
    await userEvent.click(screen.getByRole("button"));
    await userEvent.type(await screen.findByLabelText("Display name"), "Test bar");
    await userEvent.selectOptions(screen.getByLabelText(/risk profile/i), "low");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith("new-gold"));
  });
});

function Location() {
  const loc = useLocation();
  return <div data-testid="path">{`${loc.pathname}|${JSON.stringify(loc.state)}`}</div>;
}

function RecordHarness() {
  return (
    <CreateSnapshotDialog
      currency="IDR"
      snapshots={[]}
      createMutation={{ mutate: vi.fn(), reset: vi.fn(), isPending: false } as never}
      updateMutation={{ mutate: vi.fn(), reset: vi.fn(), isPending: false } as never}
    />
  );
}

function Detail() {
  return <DetailRoute render={() => <RecordHarness />} />;
}

function Go({ to, record }: { to: string; record?: boolean }) {
  const nav = useNavigate();
  return (
    <button onClick={() => nav(to, arrivalOptions(record ? { recordSnapshot: true } : undefined))}>
      go
    </button>
  );
}

function renderRoutes() {
  return renderWithProviders(
    <MemoryRouter initialEntries={["/list"]}>
      <Location />
      <Routes>
        <Route path="/list" element={<Go to="/items/n1" record />} />
        <Route path="/plain" element={<Go to="/items/n1" />} />
        <Route path="/items/:id" element={<Detail />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("Landing on the new item", () => {
  // covers: INV-PRESENTATION-20
  it("opens Record snapshot once on arrival and strips the request from history", async () => {
    renderRoutes();
    await userEvent.click(screen.getByRole("button", { name: "go" }));

    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    // The state flag is consumed: the history entry no longer asks for a re-open.
    await waitFor(() => expect(screen.getByTestId("path")).toHaveTextContent("/items/n1|null"));
  });

  // covers: INV-PRESENTATION-20
  it("does not open Record snapshot on a plain navigation", async () => {
    renderWithProviders(
      <MemoryRouter initialEntries={["/items/n1"]}>
        <Routes>
          <Route path="/items/:id" element={<Detail />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(await screen.findByTestId("snapshot-record")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

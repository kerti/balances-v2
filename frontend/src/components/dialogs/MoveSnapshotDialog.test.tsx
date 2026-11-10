// Move… from Edit snapshot (#791, ADR-0046 amendment, ADR-0058): the month stays
// read-only in the form, Move… opens a month picker with the same ceiling as
// Record snapshot, a taken target shows the replace notice before anything is
// sent, and a confirmed move is one POST and one toast with Undo.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseMutationResult } from "@tanstack/react-query";
import { toast } from "sonner";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { raisedToasts } from "@/test/sonnerMock";
import {
  EditSnapshotDialog,
  type UpdateSnapshotMutationVariables,
} from "@/components/dialogs/EditSnapshotDialog";
import { EditQuantityPriceSnapshotDialog } from "@/components/dialogs/EditQuantityPriceSnapshotDialog";
import { EditAccruedInterestSnapshotDialog } from "@/components/dialogs/EditAccruedInterestSnapshotDialog";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const idleMutation = {
  mutate: vi.fn(),
  isError: false,
  isPending: false,
  error: null,
} as unknown as UseMutationResult<unknown, unknown, UpdateSnapshotMutationVariables>;

const BASE = "/api/assets/a1/snapshots";

// October 2026 is the month being moved; September 2026 holds a value.
const snapshot = {
  id: "oct-1",
  asset_id: "a1",
  year_month: "2026-10-01T00:00:00Z",
  amount: "12500000",
  currency: "IDR",
  as_of_date: null,
  description: null,
};
const september = { ...snapshot, id: "sep-1", year_month: "2026-09-01T00:00:00Z", amount: "9000" };
const listWith = [snapshot, september];

function renderEdit(rows: unknown[] = [snapshot], snap: object = snapshot) {
  server.use(http.get(BASE, () => HttpResponse.json(rows)));
  return renderWithProviders(
    <EditSnapshotDialog
      open
      onOpenChange={vi.fn()}
      snapshot={snap as typeof snapshot}
      mutation={idleMutation}
    />,
  );
}

async function openMove() {
  await userEvent.click(screen.getByTestId("snapshot-move-open"));
  return screen.findByRole("heading", { name: "Move snapshot to another month" });
}

async function pickMonth(key: string) {
  await userEvent.click(screen.getByTestId("move_year_month"));
  await userEvent.click(await screen.findByTestId(`month-picker-cell-${key}`));
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 10, 9, 0, 0));
});
afterEach(() => vi.useRealTimers());

describe("Move… in Edit snapshot", () => {
  // covers: INV-SNAPSHOTS-15
  it("keeps the month read-only, offers Move…, and no longer tells the user to delete and re-record", () => {
    renderEdit();
    expect(screen.getByTestId("snapshot-month-locked")).toBeDisabled();
    expect(screen.getByTestId("snapshot-move-open")).toHaveTextContent("Move…");
    expect(screen.queryByText(/Wrong month/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/delete the snapshot from the row menu/i)).not.toBeInTheDocument();
  });

  // covers: INV-SNAPSHOTS-15
  it("offers Move… in the quantity and accrued-interest shapes too, against the investment route", async () => {
    const investment = {
      ...snapshot,
      asset_id: undefined,
      investment_id: "i1",
      quantity: "10",
      price_per_unit: "1250000",
      accrued_interest: "5",
    };
    server.use(http.get("/api/investments/i1/snapshots", () => HttpResponse.json([investment])));
    const view = renderWithProviders(
      <EditQuantityPriceSnapshotDialog
        open
        onOpenChange={vi.fn()}
        snapshot={investment}
        mutation={idleMutation as never}
      />,
    );
    expect(screen.getByTestId("snapshot-move-open")).toBeInTheDocument();
    view.unmount();

    let moved: unknown = null;
    server.use(
      http.post("/api/investments/i1/snapshots/oct-1/move", async ({ request }) => {
        moved = await request.json();
        return HttpResponse.json({ ...investment, id: "sep-new", replaced_id: null });
      }),
    );
    renderWithProviders(
      <EditAccruedInterestSnapshotDialog
        open
        onOpenChange={vi.fn()}
        snapshot={investment}
        mutation={idleMutation as never}
      />,
    );
    await openMove();
    await pickMonth("2026-09");
    await userEvent.click(screen.getByTestId("snapshot-move-confirm"));
    await waitFor(() => expect(moved).toEqual({ year_month: "2026-09" }));
  });

  it("is not offered for a row with no parent position id", () => {
    renderEdit([snapshot], { ...snapshot, asset_id: undefined });
    expect(screen.queryByTestId("snapshot-move-open")).not.toBeInTheDocument();
  });

  // covers: INV-SNAPSHOTS-15
  it("opens a picker with the same ceiling as Record snapshot: no month after the current one", async () => {
    renderEdit();
    await openMove();
    await userEvent.click(screen.getByTestId("move_year_month"));
    expect(await screen.findByTestId("month-picker-cell-2026-11")).toBeEnabled();
    expect(screen.getByTestId("month-picker-cell-2026-12")).toBeDisabled();
  });

  // covers: INV-SNAPSHOTS-15, INV-SOFT-DELETE-13
  it("moves to an empty month with one POST and one toast whose Undo reaches undo-move", async () => {
    const calls: string[] = [];
    let moveBody: unknown = null;
    let undoBody: unknown = null;
    server.use(
      http.post(`${BASE}/oct-1/move`, async ({ request }) => {
        calls.push("move");
        moveBody = await request.json();
        return HttpResponse.json({
          ...snapshot,
          id: "sep-new",
          year_month: "2026-08-01T00:00:00Z",
          replaced_id: null,
        });
      }),
      http.post(`${BASE}/oct-1/undo-move`, async ({ request }) => {
        calls.push("undo-move");
        undoBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderEdit();
    await openMove();
    expect(screen.queryByTestId("snapshot-move-replace-notice")).not.toBeInTheDocument();
    await pickMonth("2026-08");
    await userEvent.click(screen.getByTestId("snapshot-move-confirm"));

    await waitFor(() => expect(calls).toEqual(["move"]));
    expect(moveBody).toEqual({ year_month: "2026-08" });

    await waitFor(() => expect(raisedToasts(mockedToast)).toHaveLength(1));
    const [raised] = raisedToasts(mockedToast);
    expect(raised.message).toBe("Moved to Aug 2026");
    expect(raised.options.action?.label).toBe("Undo");

    act(() => raised.options.action?.onClick());
    await waitFor(() => expect(undoBody).toEqual({ moved_id: "sep-new", displaced_id: null }));
    expect(calls).toEqual(["move", "undo-move"]);
  });

  // covers: INV-SNAPSHOTS-15, INV-SOFT-DELETE-13
  it("shows the replace notice for a taken target and Undo names the displaced row", async () => {
    let undoBody: unknown = null;
    server.use(
      http.post(`${BASE}/oct-1/move`, () =>
        HttpResponse.json({
          ...snapshot,
          id: "sep-new",
          year_month: "2026-09-01T00:00:00Z",
          replaced_id: "sep-1",
        }),
      ),
      http.post(`${BASE}/oct-1/undo-move`, async ({ request }) => {
        undoBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderEdit(listWith);
    await openMove();
    await pickMonth("2026-09");

    // Before anything is sent, in the Record snapshot pattern.
    const notice = await screen.findByTestId("snapshot-move-replace-notice");
    expect(notice).toHaveTextContent(/Sep 2026 already has .*9,000.*moving here replaces it/);

    await userEvent.click(screen.getByTestId("snapshot-move-confirm"));
    await waitFor(() => expect(raisedToasts(mockedToast)).toHaveLength(1));
    const [raised] = raisedToasts(mockedToast);
    expect(raised.message).toBe("Moved to Sep 2026 · earlier value replaced");

    act(() => raised.options.action?.onClick());
    await waitFor(() => expect(undoBody).toEqual({ moved_id: "sep-new", displaced_id: "sep-1" }));
  });

  it("asks for a month, and refuses the snapshot's own, without sending anything", async () => {
    let sent = 0;
    server.use(
      http.post(`${BASE}/oct-1/move`, () => {
        sent++;
        return HttpResponse.json({});
      }),
    );
    renderEdit();
    await openMove();
    await userEvent.click(screen.getByTestId("snapshot-move-confirm"));
    expect(await screen.findByText("This field is required.")).toBeInTheDocument();

    await pickMonth("2026-10");
    await userEvent.click(screen.getByTestId("snapshot-move-confirm"));
    expect(await screen.findByText("Pick a different month.")).toBeInTheDocument();
    expect(sent).toBe(0);
    expect(raisedToasts(mockedToast)).toHaveLength(0);
  });

  // covers: INV-SNAPSHOTS-15
  it("shows a refused move of a close snapshot in the dialog, not as a toast", async () => {
    server.use(
      http.post(`${BASE}/oct-1/move`, () =>
        HttpResponse.json({ code: "SNAPSHOT_CLOSE_NOT_MOVABLE" }, { status: 409 }),
      ),
    );
    renderEdit();
    await openMove();
    await pickMonth("2026-09");
    await userEvent.click(screen.getByTestId("snapshot-move-confirm"));

    expect(await screen.findByTestId("form-error")).toHaveTextContent(/closing snapshot/i);
    expect(raisedToasts(mockedToast)).toHaveLength(0);
  });

  it("explains that a statement date is cleared, only when there is one", async () => {
    renderEdit([snapshot], { ...snapshot, as_of_date: "2026-10-31T00:00:00Z" });
    await openMove();
    expect(screen.getByTestId("snapshot-move-date-note")).toHaveTextContent(
      /statement date is cleared/i,
    );
  });
});

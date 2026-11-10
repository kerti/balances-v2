// Record snapshot against the real hooks and a mocked network (#788, ADR-0058):
// recording a month that already has a value is one PATCH of the live row, never
// a second POST, and the single toast offers Undo through the replace-undo route.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { raisedToasts } from "@/test/sonnerMock";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { useCreateSnapshot, useUpdateSnapshot } from "@/hooks/useAssetSnapshots";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const snapshots = [
  {
    id: "old-1",
    year_month: "2026-11-01T00:00:00Z",
    amount: "12500000",
    as_of_date: null,
    description: null,
  },
];

function Harness() {
  const createMutation = useCreateSnapshot("a1");
  const updateMutation = useUpdateSnapshot("a1");
  return (
    <CreateSnapshotDialog
      currency="IDR"
      snapshots={snapshots}
      createMutation={createMutation}
      updateMutation={updateMutation}
    />
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 10, 9, 0, 0));
});
afterEach(() => vi.useRealTimers());

describe("Record snapshot replace round trip", () => {
  // covers: INV-SNAPSHOTS-12
  it("replaces the recorded month with one PATCH, no POST, and the toast Undo reaches the undo route", async () => {
    const calls: string[] = [];
    let patched: unknown = null;
    let undone: unknown = null;
    server.use(
      http.post("/api/assets/a1/snapshots", () => {
        calls.push("POST create");
        return HttpResponse.json({ code: "SNAPSHOT_MONTH_EXISTS" }, { status: 409 });
      }),
      http.patch("/api/assets/a1/snapshots/old-1", async ({ request }) => {
        calls.push("PATCH old-1");
        patched = await request.json();
        return HttpResponse.json({ id: "new-1", replaced_id: "old-1" });
      }),
      http.post("/api/assets/a1/snapshots/old-1/undo-replace", async ({ request }) => {
        calls.push("POST undo-replace");
        undone = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderWithProviders(<Harness />);

    await userEvent.click(screen.getByRole("button", { name: "Record snapshot" }));
    // The notice is there before anything is sent.
    expect(screen.getByTestId("snapshot-replace-notice")).toHaveTextContent(/saving replaces it/);
    await userEvent.clear(screen.getByLabelText(/Amount/));
    await userEvent.type(screen.getByLabelText(/Amount/), "13000000");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));

    await waitFor(() => expect(calls).toEqual(["PATCH old-1"]));
    expect(patched).toEqual(expect.objectContaining({ amount: "13000000", currency: "IDR" }));

    // Exactly one toast, the replaced one, carrying Undo.
    await waitFor(() => expect(raisedToasts(mockedToast)).toHaveLength(1));
    const [raised] = raisedToasts(mockedToast);
    expect(raised.message).toBe("Saved · earlier value replaced");
    expect(raised.options.action?.label).toBe("Undo");

    act(() => raised.options.action?.onClick());
    await waitFor(() => expect(undone).toEqual({ replacement_id: "new-1" }));
    expect(calls).toEqual(["PATCH old-1", "POST undo-replace"]);
  });
});

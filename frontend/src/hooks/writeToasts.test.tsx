// One toast per user-started write (ADR-0058 §3-§5): a create says Added, an edit
// that replaced a value offers Undo naming both rows, a delete offers Undo, a
// bulk save summarises, and a failed write with no form open is a red toast
// carrying the server's code. Sonner is mocked so the calls can be counted.
import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { renderHook, act, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import { server } from "@/test/server";
import { makeTestQueryClient } from "@/test/renderWithProviders";
import { raisedToasts } from "@/test/sonnerMock";
import { useCreateBankAccount } from "@/hooks/useBankAccounts";
import { useUpdateSnapshot, useDeleteSnapshot } from "@/hooks/useAssetSnapshots";
import { useDeleteTag } from "@/hooks/useTags";
import { useBulkSaveSnapshots } from "@/hooks/useBulkEntry";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={makeTestQueryClient()}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  vi.clearAllMocks();
});

// covers: INV-PRESENTATION-17, INV-PRESENTATION-18
describe("one toast per write", () => {
  it("a create raises a single plain Added toast with no action", async () => {
    server.use(http.post("/api/bank-accounts", () => HttpResponse.json({ id: "a1" })));
    const { result } = renderHook(() => useCreateBankAccount(), { wrapper });

    await act(() =>
      result.current.mutateAsync({
        display_name: "Everyday",
        description: null,
        ownership_type: "joint",
        sole_owner_user_id: null,
        native_currency: "IDR",
        bank_name: "Test Bank",
        account_number: "1",
        account_type: "savings",
        entry_type: "acquired",
        tag_id: null,
      }),
    );

    const raised = raisedToasts(mockedToast);
    expect(raised).toHaveLength(1);
    expect(raised[0].message).toBe("Added");
    expect(raised[0].options.duration).toBe(4000);
    expect(raised[0].options.action).toBeUndefined();
    expect(raised[0].options.className).toBeUndefined();
  });

  it("an edit that replaced a value offers Undo, which names both snapshot ids", async () => {
    let undone: unknown = null;
    server.use(
      http.patch("/api/assets/a1/snapshots/old-1", () =>
        HttpResponse.json({ id: "new-1", replaced_id: "old-1" }),
      ),
      http.post("/api/assets/a1/snapshots/old-1/undo-replace", async ({ request }) => {
        undone = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { result } = renderHook(() => useUpdateSnapshot("a1"), { wrapper });

    await act(() =>
      result.current.mutateAsync({
        snapshotId: "old-1",
        payload: { amount: "10", currency: "IDR", as_of_date: null, description: null },
      }),
    );

    const raised = raisedToasts(mockedToast);
    expect(raised).toHaveLength(1);
    expect(raised[0].options.duration).toBe(8000);
    // The countdown bar runs for exactly the toast's life.
    expect(raised[0].options.className).toBe("undo-countdown");
    expect(raised[0].options.style).toEqual({ "--undo-countdown-ms": "8000ms" });
    expect(raised[0].options.action?.label).toBe("Undo");

    act(() => raised[0].options.action?.onClick());
    await waitFor(() => expect(undone).toEqual({ replacement_id: "new-1" }));
    await waitFor(() =>
      expect(mockedToast.success).toHaveBeenCalledWith("Undone", expect.anything()),
    );
  });

  it("an identical edit (nothing replaced) is a plain Saved toast", async () => {
    server.use(
      http.patch("/api/assets/a1/snapshots/s1", () =>
        HttpResponse.json({ id: "s1", replaced_id: null }),
      ),
    );
    const { result } = renderHook(() => useUpdateSnapshot("a1"), { wrapper });

    await act(() =>
      result.current.mutateAsync({
        snapshotId: "s1",
        payload: { amount: "10", currency: "IDR", as_of_date: null, description: null },
      }),
    );

    const raised = raisedToasts(mockedToast);
    expect(raised).toHaveLength(1);
    expect(raised[0].message).toBe("Saved");
    expect(raised[0].options.action).toBeUndefined();
  });

  it("a delete raises one Undo toast, and Undo posts undo-delete", async () => {
    let undoCalls = 0;
    server.use(
      http.delete("/api/assets/a1/snapshots/s1", () => new HttpResponse(null, { status: 204 })),
      http.post("/api/assets/a1/snapshots/s1/undo-delete", () => {
        undoCalls += 1;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { result } = renderHook(() => useDeleteSnapshot("a1"), { wrapper });

    await act(() => result.current.mutateAsync("s1"));

    const raised = raisedToasts(mockedToast);
    expect(raised).toHaveLength(1);
    expect(raised[0].message).toBe("Snapshot deleted");
    expect(raised[0].options.duration).toBe(8000);

    act(() => raised[0].options.action?.onClick());
    await waitFor(() => expect(undoCalls).toBe(1));
  });

  it("a tag delete is a plain toast with no Undo", async () => {
    server.use(http.delete("/api/tags/t1", () => new HttpResponse(null, { status: 204 })));
    const { result } = renderHook(() => useDeleteTag(), { wrapper });

    await act(() => result.current.mutateAsync("t1"));

    const raised = raisedToasts(mockedToast);
    expect(raised).toHaveLength(1);
    expect(raised[0].message).toBe("Tag deleted");
    expect(raised[0].options.action).toBeUndefined();
  });

  it("a bulk save raises one summary toast with no Undo", async () => {
    server.use(
      http.post("/api/assets/snapshots/bulk", () =>
        HttpResponse.json({ written: 7, replaced: 2, unchanged: 0 }),
      ),
    );
    const { result } = renderHook(
      () =>
        useBulkSaveSnapshots({
          group: "assets",
          apiBase: "/api/assets/snapshots",
          idField: "asset_id",
          invalidateKeys: [],
        }),
      { wrapper },
    );

    await act(() =>
      result.current.mutateAsync({ year_month: "2026-05", as_of_date: "2026-05-31", rows: [] }),
    );

    const raised = raisedToasts(mockedToast);
    expect(raised).toHaveLength(1);
    expect(raised[0].message).toBe("Saved 7 · 2 replaced earlier values");
    expect(raised[0].options.action).toBeUndefined();
  });

  it("a failed delete with no form open is one red toast carrying the localised code", async () => {
    server.use(
      http.delete("/api/assets/a1/snapshots/s1", () =>
        HttpResponse.json({ code: "UNDO_SLOT_TAKEN" }, { status: 409 }),
      ),
    );
    const { result } = renderHook(() => useDeleteSnapshot("a1"), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("s1").catch(() => undefined);
    });

    expect(mockedToast.error).toHaveBeenCalledTimes(1);
    expect(mockedToast.error).toHaveBeenCalledWith(
      "Can't undo: something else now fills that spot (the same month or rate). Remove or edit it first.",
    );
    expect(raisedToasts(mockedToast)).toHaveLength(1);
  });

  it("a refused Undo is a red toast, not the generic message", async () => {
    server.use(
      http.delete("/api/assets/a1/snapshots/s1", () => new HttpResponse(null, { status: 204 })),
      http.post("/api/assets/a1/snapshots/s1/undo-delete", () =>
        HttpResponse.json({ code: "UNDO_SLOT_TAKEN" }, { status: 409 }),
      ),
    );
    const { result } = renderHook(() => useDeleteSnapshot("a1"), { wrapper });
    await act(() => result.current.mutateAsync("s1"));

    act(() => raisedToasts(mockedToast)[0].options.action?.onClick());

    await waitFor(() => expect(mockedToast.error).toHaveBeenCalledTimes(1));
    expect(vi.mocked(mockedToast.error).mock.calls[0][0]).toMatch(/^Can't undo/);
  });
});

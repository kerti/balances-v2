import { afterEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { ApiError } from "@/api/client";
import { makeTestQueryClient } from "@/test/renderWithProviders";
import { useBulkSaveSnapshots } from "@/hooks/useBulkEntry";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

const cfg = {
  group: "asset",
  apiBase: "/api/assets/snapshots",
  idField: "asset_id",
  invalidateKeys: [],
} as unknown as Parameters<typeof useBulkSaveSnapshots>[0];

const args = { year_month: "2026-03", as_of_date: "", rows: [] };

function save(status: number, body: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
  const client = makeTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return renderHook(() => useBulkSaveSnapshots(cfg), { wrapper });
}

afterEach(() => vi.restoreAllMocks());

describe("useBulkSaveSnapshots on a 422", () => {
  it("returns the per-row rejections as data", async () => {
    const { result } = save(422, { errors: [{ asset_id: "a1", code: "SNAPSHOT_FUTURE_DATE" }] });
    result.current.mutate(args);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({
      ok: false,
      errors: [{ position_id: "a1", code: "SNAPSHOT_FUTURE_DATE" }],
    });
  });

  // covers: INV-CONTRACT-06
  it("throws the VALUE_OUT_OF_RANGE envelope instead of reading it as per-row data", async () => {
    const { result } = save(422, { code: "VALUE_OUT_OF_RANGE" });
    result.current.mutate(args);
    await waitFor(() => expect(result.current.isError).toBe(true));
    const err = result.current.error as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.body).toEqual({ code: "VALUE_OUT_OF_RANGE" });
  });
});

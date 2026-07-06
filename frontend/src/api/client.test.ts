import { describe, it, expect, vi, afterEach } from "vitest";

import { api, ApiError, isEnvelope } from "@/api/client";

describe("api", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("attaches an abort signal to every request (#360 request timeout)", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.signal).toBeInstanceOf(AbortSignal);
      expect(init?.signal?.aborted).toBe(false);
      return new Response(null, { status: 204 });
    });
    vi.stubGlobal("fetch", fetchMock);

    await api("/api/whatever");

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("combines a caller-supplied signal with the default timeout signal", async () => {
    const controller = new AbortController();
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect(init?.signal).toBeInstanceOf(AbortSignal);
      return new Response(null, { status: 204 });
    });
    vi.stubGlobal("fetch", fetchMock);

    await api("/api/whatever", { signal: controller.signal });

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("throws ApiError with the parsed envelope on a non-2xx response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ code: "NOT_FOUND" }), {
            status: 404,
            headers: { "Content-Type": "application/json" },
          }),
      ),
    );

    await expect(api("/api/missing")).rejects.toSatisfy((err: unknown) => {
      expect(err).toBeInstanceOf(ApiError);
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(404);
      expect(isEnvelope(apiErr.body)).toBe(true);
      return true;
    });
  });
});

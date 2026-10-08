import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "@/api/client";
import { OfflineBanner } from "@/components/shell/OfflineBanner";

afterEach(() => {
  vi.unstubAllGlobals();
});

// covers: INV-PRESENTATION-12
describe("OfflineBanner", () => {
  it("renders nothing while connected", () => {
    render(<OfflineBanner />);
    expect(screen.queryByTestId("offline-banner")).not.toBeInTheDocument();
  });

  it("follows the browser's offline and online events", async () => {
    render(<OfflineBanner />);

    act(() => {
      window.dispatchEvent(new Event("offline"));
    });
    expect(await screen.findByTestId("offline-banner")).toHaveTextContent(
      "You're offline — Balances needs a connection.",
    );

    act(() => {
      window.dispatchEvent(new Event("online"));
    });
    await waitFor(() => expect(screen.queryByTestId("offline-banner")).not.toBeInTheDocument());
  });

  it("appears on a failed request and clears on the next response", async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("Failed to fetch"))
      .mockResolvedValueOnce(new Response(JSON.stringify({}), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<OfflineBanner />);

    await act(() => expect(api("/api/session")).rejects.toThrow());
    expect(await screen.findByTestId("offline-banner")).toBeInTheDocument();

    await act(() => api("/api/session"));
    await waitFor(() => expect(screen.queryByTestId("offline-banner")).not.toBeInTheDocument());
  });

  it("ignores a request the caller cancelled itself", async () => {
    const abort = new AbortController();
    abort.abort();
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new DOMException("aborted", "AbortError")));
    render(<OfflineBanner />);

    await act(() => expect(api("/api/session", { signal: abort.signal })).rejects.toThrow());
    expect(screen.queryByTestId("offline-banner")).not.toBeInTheDocument();
  });
});

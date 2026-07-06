// Thin fetch wrapper. Adds JSON content-type for requests with a body,
// throws ApiError on non-2xx, and returns null for 204 No Content. All API
// calls go through the same-origin Vite proxy so the session cookie travels
// automatically.

// ErrorEnvelope is the wire shape every 4xx/5xx body from internal/* ships
// per ADR-0027. `code` is the stable contract; `args` is an optional flat
// map of interpolation values passed straight into react-i18next's t()
// alongside the matching `errors:code.<CODE>` catalog entry. The OAuth
// callback (redirects) and the per-row snapshot-importer 422 body are the
// documented exceptions and never produce this shape.
export type ErrorEnvelope = {
  code: string;
  args?: Record<string, unknown>;
};

// isEnvelope narrows ApiError.body — which the client parses opportunistically
// as JSON and otherwise falls back to a raw string — to the typed envelope.
// Callers use this guard to decide whether to look up a catalog key or fall
// through to the generic UNKNOWN copy.
export function isEnvelope(body: unknown): body is ErrorEnvelope {
  return (
    typeof body === "object" &&
    body !== null &&
    typeof (body as { code?: unknown }).code === "string"
  );
}

export class ApiError extends Error {
  status: number;
  body: ErrorEnvelope | string | undefined;

  constructor(status: number, message: string, body?: ErrorEnvelope | string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.body = body;
  }
}

// DEFAULT_TIMEOUT_MS bounds every request against a hung connection or a
// backend that never responds (#360) — nothing else on the client side was
// giving up on a stalled fetch. Comfortably under the server's own
// HTTP_READ_TIMEOUT/HTTP_WRITE_TIMEOUT defaults (30s/60s) so the client times
// out and surfaces an error before the server would have anyway.
const DEFAULT_TIMEOUT_MS = 20_000;

export async function api<T = unknown>(input: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const timeoutSignal = AbortSignal.timeout(DEFAULT_TIMEOUT_MS);
  const signal = init.signal ? AbortSignal.any([init.signal, timeoutSignal]) : timeoutSignal;

  const res = await fetch(input, { ...init, headers, signal });

  if (!res.ok) {
    let body: ErrorEnvelope | string | undefined;
    try {
      const parsed = await res.json();
      body = isEnvelope(parsed) ? parsed : undefined;
    } catch {
      try {
        body = await res.text();
      } catch {
        /* swallow */
      }
    }
    throw new ApiError(res.status, res.statusText || `request failed (${res.status})`, body);
  }

  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

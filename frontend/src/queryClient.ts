import { MutationCache, QueryClient } from "@tanstack/react-query";
import { ApiError } from "@/api/client";

// React Query's default retries any failed query 3x regardless of status. A
// 4xx means the server rejected the request itself (bad input, not found,
// unauthorized) — identical bytes will fail identically, so retrying just
// delays the error reaching the UI. Network errors and 5xx are transient and
// keep the default retry budget (#360).
export function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < 3;
}

// Any successful write may change a monthly-report input — a snapshot,
// transaction, income event, position metadata/lifecycle, or FX rate. Reports
// regenerate lazily on read (ADR-0006), so invalidating ['reports'] after every
// mutation keeps the dashboard's net worth fresh. Done globally rather than per
// hook because ADR-0006 warns that enumerating every input drifts silently when
// one is missed — the same fragility applies on the client. Cheap for a
// single-household app: the refetch's server-side regen is a no-op when nothing
// actually went stale, and only fires when the dashboard query is mounted.
export function createQueryClient(): QueryClient {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: shouldRetryQuery },
    },
    mutationCache: new MutationCache({
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["reports"] });
      },
    }),
  });
  return queryClient;
}

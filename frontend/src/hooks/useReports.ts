import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import i18n from "@/i18n";
import { withLoadingToast } from "@/lib/writeToast";
import type { MonthlyReport } from "@/api/types";

// The dashboard's net-worth series. The backend regenerates stale months on
// read (ADR-0006), so this is a plain fetch; mutations elsewhere (snapshots,
// positions) should invalidate ['reports'] to pull a fresh series.
export function useReports() {
  return useQuery({
    queryKey: ["reports"],
    queryFn: () => api<MonthlyReport[]>("/api/reports"),
    staleTime: 10_000,
  });
}

// Manual rebuild (ADR-0006). The data-driven staleness watermark can't detect
// engine-code changes, so a force-rebuild is the escape hatch. Two scopes:
// rebuild-all (engine/FX changes that ripple across history) and rebuild-month
// (surgical fix). The dashboard re-reads via the global ['reports'] invalidation
// in main.tsx (fires on every mutation success).
export function useRebuildReports() {
  const rebuildAll = useMutation({
    mutationFn: () =>
      withLoadingToast(
        i18n.t("dashboard:rebuild.rebuilding"),
        () => api("/api/reports/rebuild", { method: "POST" }),
        () => i18n.t("dashboard:rebuild.doneAll"),
        { fallbackError: i18n.t("dashboard:rebuild.failed") },
      ),
  });

  const rebuildMonth = useMutation({
    // yearMonth is the report's full ISO date; the endpoint accepts YYYY-MM.
    mutationFn: (yearMonth: string) =>
      withLoadingToast(
        i18n.t("dashboard:rebuild.rebuilding"),
        () => api(`/api/reports/${yearMonth.slice(0, 7)}/rebuild`, { method: "POST" }),
        () => i18n.t("dashboard:rebuild.doneMonth"),
        { fallbackError: i18n.t("dashboard:rebuild.failed") },
      ),
  });

  return { rebuildAll, rebuildMonth };
}

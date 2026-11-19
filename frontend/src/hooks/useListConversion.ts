import { useMemo } from "react";
import { useHubConversion } from "@/hooks/useGroupBreakdown";
import { latestMonth, listMissingFx, listShare } from "@/lib/groupBreakdown";
import type { GroupBreakdown, MissingFx } from "@/api/types";

// Which slice of the engine's group breakdown a position list headlines: the
// whole group (receivables) or one subtype of it (bank accounts, stocks, …).
export type ListBreakdown = {
  group: GroupBreakdown["group"];
  subtype?: string;
};

export type ListConversion =
  | { mode: "native" }
  | { mode: "loading" }
  | {
      mode: "converted";
      currency: string;
      total: string;
      yearMonth: string;
      missingFx: MissingFx[];
      breakdown: GroupBreakdown;
    };

// A list headline in the reporting currency (#937), from the same engine pass as
// the dashboard. Mirrors the hubs' rule (useHubConversion): only a
// multi-currency household holding something outside the reporting currency
// converts; every other household keeps the native headline exactly as before.
// While the breakdown loads, or if it fails, nothing wrong is shown: loading
// renders no headline, an error falls back to the native one.
export function useListConversion(
  breakdown: ListBreakdown,
  positions: { id: string; currency: string; status: string }[],
): ListConversion {
  const held = useMemo(
    () => [
      ...new Set(
        positions.filter((p) => p.status === "active" && p.currency).map((p) => p.currency),
      ),
    ],
    [positions],
  );
  const { converted, reporting, breakdown: q } = useHubConversion(breakdown.group, held);
  const month = latestMonth(q.data);
  if (!converted) return { mode: "native" };
  if (q.data && month) {
    return {
      mode: "converted",
      currency: reporting,
      total: listShare(month, breakdown.subtype),
      yearMonth: month.year_month,
      missingFx: listMissingFx(
        month,
        positions.map((p) => p.id),
      ),
      breakdown: q.data,
    };
  }
  return q.isPending ? { mode: "loading" } : { mode: "native" };
}

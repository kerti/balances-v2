import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { useSession } from "@/hooks/useSession";
import type { GroupBreakdown } from "@/api/types";

type HubGroup = GroupBreakdown["group"];

// Kept under the ['reports'] prefix so the global post-write invalidation in
// main.tsx sweeps it with the dashboard's series it must always agree with.
export const groupBreakdownKey = (group: HubGroup) => ["reports", "group", group] as const;

// One overview hub's value in the reporting currency, from the same engine pass
// as the dashboard's reports (ADR-0002, ADR-0022 amendment, #845). The hub
// total therefore *is* the dashboard's group total, not a client-side
// re-conversion.
export function useGroupBreakdown(group: HubGroup, enabled: boolean) {
  return useQuery({
    queryKey: groupBreakdownKey(group),
    queryFn: () => api<GroupBreakdown>(`/api/reports/groups/${group}`),
    enabled,
    staleTime: 10_000,
  });
}

// A hub renders its converted set only for a multi-currency household that
// actually holds something outside the reporting currency; otherwise native
// and converted are the same numbers and the hub stays exactly as it was.
export function useHubConversion(group: HubGroup, heldCurrencies: string[]) {
  const me = useSession();
  const reporting = me.data?.reporting_currency ?? "";
  const converted =
    !!me.data?.multi_currency_enabled && heldCurrencies.some((c) => c !== reporting);
  const breakdown = useGroupBreakdown(group, converted);
  return { converted, reporting, breakdown };
}

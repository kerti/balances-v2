import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";

// Updates the Household's display name (#265, any member), reporting currency,
// and multi-currency toggle (ADR-0002) as one full-replace PATCH. Refreshes the
// session (which carries the settings); the dashboard re-converts via the
// global ['reports'] invalidation in main.tsx. The backend returns 409 when
// turning the toggle off while foreign-currency positions still exist —
// surfaced to the caller as an ApiError.
export function useUpdateHouseholdSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (p: {
      display_name: string;
      reporting_currency: string;
      multi_currency_enabled: boolean;
    }) =>
      api("/api/household/settings", {
        method: "PATCH",
        body: JSON.stringify(p),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["session"] });
    },
  });
}

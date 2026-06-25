import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { HouseholdMember } from "@/api/types";

export function useHouseholdMembers() {
  return useQuery({
    queryKey: ["household-members"],
    queryFn: () => api<HouseholdMember[]>("/api/household/members"),
    // Members rarely change; keep them cached longer than position data.
    staleTime: 5 * 60_000,
  });
}

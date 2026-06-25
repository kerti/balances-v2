import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { Tag, TagBreakdownRow, TagGroup } from "@/api/types";

export type TagWritePayload = { name: string; color: string };

export type AssignTagPayload = {
  group: TagGroup;
  position_id: string;
  tag_id: string | null; // null unassigns
};

export function useTags() {
  return useQuery({
    queryKey: ["tags"],
    queryFn: () => api<Tag[]>("/api/tags"),
    staleTime: 30_000,
  });
}

export function useTagBreakdown() {
  return useQuery({
    queryKey: ["tags", "breakdown"],
    queryFn: () => api<TagBreakdownRow[]>("/api/tags/breakdown"),
    staleTime: 10_000,
  });
}

export function useCreateTag() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: TagWritePayload) =>
      api<Tag>("/api/tags", { method: "POST", body: JSON.stringify(payload) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tags"] }),
  });
}

export function useUpdateTag(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: TagWritePayload) =>
      api<Tag>(`/api/tags/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["tags"] });
      // A recolour/rename changes how the breakdown renders.
      qc.invalidateQueries({ queryKey: ["tags", "breakdown"] });
    },
  });
}

export function useDeleteTag() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/tags/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      // Delete clears the tag off every position server-side, so any list /
      // detail / breakdown that showed it is now stale.
      qc.invalidateQueries();
    },
  });
}

// useAssignTag sets or clears the Tag on a Position. Invalidates broadly: the
// breakdown changes, and the affected position's list/detail caches carry the
// old tag_id.
export function useAssignTag() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: AssignTagPayload) =>
      api("/api/tags/assignments", {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["tags", "breakdown"] });
    },
  });
}

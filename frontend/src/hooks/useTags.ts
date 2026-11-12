import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { toast } from "sonner";
import i18n from "@/i18n";
import { PLAIN_TOAST_MS, toastSaved, toastWriteError } from "@/lib/writeToast";
import type { Tag, TagBreakdownRow, TagGroup, TaggableItem } from "@/api/types";

export type TagWritePayload = { name: string; color: string };

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

export function useTaggableItems() {
  return useQuery({
    queryKey: ["tags", "items"],
    queryFn: () => api<TaggableItem[]>("/api/tags/items"),
    staleTime: 10_000,
  });
}

export type AssignTagsBulkPayload = {
  tag_id: string | null; // null clears
  items: { group: TagGroup; position_id: string }[];
};

// useAssignTagsBulk sets (or clears) the Tag on many items in one transaction
// (#853). The caller raises the single toast: Saved for an add, Undo for a
// remove. Invalidates every query: a Tag shows on every list and detail page.
export function useAssignTagsBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: AssignTagsBulkPayload) =>
      api("/api/tags/assignments", { method: "PUT", body: JSON.stringify(payload) }),
    onSuccess: () => {
      qc.invalidateQueries();
    },
  });
}

export function useCreateTag() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: TagWritePayload) =>
      api<Tag>("/api/tags", { method: "POST", body: JSON.stringify(payload) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["tags"] });
      toastSaved("added");
    },
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
      toastSaved();
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
      // No Undo: the delete clears every item's assignment irrecoverably, which
      // is why the card asks first (ADR-0058 §3, amended 2026-11-10).
      toast.success(i18n.t("common:toast.deleted.tag"), { duration: PLAIN_TOAST_MS });
    },
    onError: toastWriteError,
  });
}

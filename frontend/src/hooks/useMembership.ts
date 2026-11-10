import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/api/client";
import i18n from "@/i18n";
import { PLAIN_TOAST_MS, toastWriteError } from "@/lib/writeToast";
import type { Membership } from "@/api/types";

export const MEMBERSHIP_KEY = ["household-membership"] as const;

// The Settings members table: every member plus the household's pending,
// unexpired invitations (#847).
export function useMembership() {
  return useQuery({
    queryKey: MEMBERSHIP_KEY,
    queryFn: () => api<Membership>("/api/household/membership"),
    staleTime: 30_000,
  });
}

// Revoke deletes the unused invitation, so its link stops working at once. No
// Undo: re-inviting is the recovery (ADR-0058 §3), so the toast is a plain one.
// A failure (e.g. someone revoked or the invitee accepted a moment ago) is a red
// toast with the server's localised message — no form is open (§5) — and the
// table refetches so it stops offering a row that is already gone.
export function useRevokeInvitation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/invitations/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success(i18n.t("settings:members.revoked"), { duration: PLAIN_TOAST_MS });
    },
    onError: toastWriteError,
    onSettled: () => qc.invalidateQueries({ queryKey: MEMBERSHIP_KEY }),
  });
}

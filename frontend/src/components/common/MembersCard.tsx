import { useState, type ReactNode } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { MoreHorizontal, Trash2, UserCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  useMenuOpenOnClick,
} from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { api } from "@/api/client";
import { useAuthMethods } from "@/hooks/useAuthMethods";
import { useCopyLink } from "@/hooks/useCopyLink";
import { useIsMobile } from "@/hooks/use-mobile";
import { useMembership, useRevokeInvitation } from "@/hooks/useMembership";
import { useSession } from "@/hooks/useSession";
import { formatDate, formatDateTime } from "@/lib/format";
import { toastWriteError } from "@/lib/writeToast";
import type { MembershipInvitation, MembershipMember } from "@/api/types";

type ReactivateResp = {
  email: string;
  set_password_url: string;
  expires_at: string;
};

const pill = "rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap";

function YouTag() {
  const { t } = useTranslation("settings");
  return (
    <span className={`${pill} bg-primary/10 text-primary`} data-testid="member-you-tag">
      {t("members.you")}
    </span>
  );
}

function DormantTag() {
  const { t } = useTranslation("settings");
  return (
    <span
      className={`${pill} border border-warning/40 bg-warning-surface text-foreground`}
      data-testid="member-dormant-tag"
    >
      {t("members.dormant")}
    </span>
  );
}

// The row's ⋯ menu. It is the one place a row's actions live (the convention for
// every list row, #840); the trigger is `size-11` on a phone so it meets the tap
// floor (INV-PRESENTATION-08).
function RowMenu({
  label,
  testId,
  mobile,
  children,
}: {
  label: string;
  testId: string;
  mobile: boolean;
  children: ReactNode;
}) {
  // The ⋮ is dead on iOS without this — see `useMenuOpenOnClick` (#572).
  const menu = useMenuOpenOnClick();
  return (
    <DropdownMenu {...menu.rootProps}>
      <DropdownMenuTrigger asChild>
        <Button
          variant={mobile ? "outline" : "ghost"}
          size="icon"
          className={mobile ? "size-11 shrink-0" : undefined}
          aria-label={label}
          data-testid={testId}
          {...menu.triggerProps}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">{children}</DropdownMenuContent>
    </DropdownMenu>
  );
}

// MembersCard is Settings ▸ Membership's table (#847): who is in the household,
// and which invitations are still waiting. Members come first; pending invitations
// follow in the same card, shown only while there are any (an expired invitation is
// not pending and is never listed). Reactivating a dormant member (ADR-0039) lives
// in that member's ⋯ menu — founder-only on a local-auth instance, enforced again by
// the backend — and revoking an invitation in the invitation's.
//
// Layout divergence (ADR-0050): `useIsMobile` mounts stacked cards on a phone and a
// table on desktop, so only one tree is ever in the DOM. Both share the row testids.
export function MembersCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const { data: methods } = useAuthMethods();
  const { data, isPending, isError } = useMembership();
  const revoke = useRevokeInvitation();
  const isMobile = useIsMobile();

  const [revoking, setRevoking] = useState<MembershipInvitation | null>(null);
  const [reactivated, setReactivated] = useState<ReactivateResp | null>(null);

  const reactivate = useMutation({
    mutationFn: (userId: string) =>
      api<ReactivateResp>("/api/auth/local/reactivation", {
        method: "POST",
        body: JSON.stringify({ user_id: userId }),
      }),
    onSuccess: setReactivated,
    onError: toastWriteError,
  });

  const canReactivate = Boolean(me?.is_founder && methods?.local);
  const members = data?.members ?? [];
  const invitations = data?.invitations ?? [];

  const memberMenu = (m: MembershipMember, mobile: boolean) =>
    canReactivate && m.dormant && m.id !== me?.id ? (
      <RowMenu
        label={t("members.memberActions", { name: m.email })}
        testId={`member-actions-${m.email}`}
        mobile={mobile}
      >
        <DropdownMenuItem
          data-testid={`reactivate-${m.email}`}
          disabled={reactivate.isPending}
          onClick={() => reactivate.mutate(m.id)}
        >
          <UserCheck />
          {t("members.reactivate")}
        </DropdownMenuItem>
      </RowMenu>
    ) : null;

  const inviteMenu = (inv: MembershipInvitation, mobile: boolean) => (
    <RowMenu
      label={t("members.inviteActions", { email: inv.invited_email })}
      testId={`invite-actions-${inv.invited_email}`}
      mobile={mobile}
    >
      <DropdownMenuItem
        variant="destructive"
        data-testid={`revoke-${inv.invited_email}`}
        onClick={() => setRevoking(inv)}
      >
        <Trash2 />
        {t("members.revoke")}
      </DropdownMenuItem>
    </RowMenu>
  );

  return (
    <Card data-testid="members-card">
      <CardHeader>
        <CardTitle>{t("members.title")}</CardTitle>
        <CardDescription>{t("members.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}
        {isError && <p className="text-sm text-destructive">{t("members.loadFailed")}</p>}

        {data && (
          <section aria-label={t("members.membersHeading")} className="space-y-2">
            {isMobile ? (
              <div className="space-y-2" data-testid="member-cards">
                {members.map((m) => (
                  <MemberCard
                    key={m.id}
                    member={m}
                    isYou={m.id === me?.id}
                    menu={memberMenu(m, true)}
                  />
                ))}
              </div>
            ) : (
              <Table data-testid="member-table">
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("members.name")}</TableHead>
                    <TableHead>{t("members.email")}</TableHead>
                    <TableHead>{t("members.joined")}</TableHead>
                    <TableHead className="w-12">
                      <span className="sr-only">{t("members.actions")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {members.map((m) => (
                    <TableRow key={m.id} data-testid={`member-row-${m.email}`}>
                      <TableCell>
                        <MemberName member={m} isYou={m.id === me?.id} />
                      </TableCell>
                      <TableCell className="text-muted-foreground">{m.email}</TableCell>
                      <TableCell>{formatDate(m.joined_at)}</TableCell>
                      <TableCell className="text-right">{memberMenu(m, false)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </section>
        )}

        {invitations.length > 0 && (
          <section aria-label={t("members.pendingHeading")} className="space-y-2">
            <div className="space-y-1">
              <h3 className="text-sm font-medium">{t("members.pendingHeading")}</h3>
              <p className="text-sm text-muted-foreground">{t("members.pendingHint")}</p>
            </div>
            {isMobile ? (
              <div className="space-y-2" data-testid="invite-cards">
                {invitations.map((inv) => (
                  <InviteCard key={inv.id} invitation={inv} menu={inviteMenu(inv, true)} />
                ))}
              </div>
            ) : (
              <Table data-testid="invite-table">
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("members.email")}</TableHead>
                    <TableHead>{t("members.sent")}</TableHead>
                    <TableHead>{t("members.expires")}</TableHead>
                    <TableHead className="w-12">
                      <span className="sr-only">{t("members.actions")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {invitations.map((inv) => (
                    <TableRow key={inv.id} data-testid={`invite-row-${inv.invited_email}`}>
                      <TableCell>{inv.invited_email}</TableCell>
                      <TableCell>{formatDate(inv.sent_at)}</TableCell>
                      <TableCell>{formatDate(inv.expires_at)}</TableCell>
                      <TableCell className="text-right">{inviteMenu(inv, false)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </section>
        )}
      </CardContent>

      {/* Destructive confirm, no Undo (ADR-0058 §3): re-inviting is the recovery. */}
      <ConfirmDialog
        open={revoking !== null}
        onOpenChange={(open) => {
          if (!open) setRevoking(null);
        }}
        title={t("members.revokeTitle", { email: revoking?.invited_email ?? "" })}
        description={t("members.revokeDescription")}
        confirmLabel={t("members.revoke")}
        destructive
        pending={revoke.isPending}
        onConfirm={() => {
          if (revoking) revoke.mutate(revoking.id, { onSettled: () => setRevoking(null) });
        }}
      />

      {reactivated && (
        <ReactivationLinkDialog result={reactivated} onClose={() => setReactivated(null)} />
      )}
    </Card>
  );
}

function MemberName({ member, isYou }: { member: MembershipMember; isYou: boolean }) {
  return (
    <span className="flex flex-wrap items-center gap-2">
      <span className="font-medium">{member.nickname ?? member.display_name}</span>
      {isYou && <YouTag />}
      {member.dormant && <DormantTag />}
    </span>
  );
}

// Mobile leaf: one stacked card per member, ≥44px menu trigger.
function MemberCard({
  member,
  isYou,
  menu,
}: {
  member: MembershipMember;
  isYou: boolean;
  menu: ReactNode;
}) {
  const { t } = useTranslation("settings");
  return (
    <div
      className="flex items-center gap-3 rounded-lg border p-3"
      data-testid={`member-row-${member.email}`}
    >
      <div className="min-w-0 flex-1 space-y-1">
        <MemberName member={member} isYou={isYou} />
        <div className="text-sm break-all text-muted-foreground">{member.email}</div>
        <div className="text-sm text-muted-foreground">
          {t("members.joinedOn", { date: formatDate(member.joined_at) })}
        </div>
      </div>
      {menu}
    </div>
  );
}

function InviteCard({ invitation, menu }: { invitation: MembershipInvitation; menu: ReactNode }) {
  const { t } = useTranslation("settings");
  return (
    <div
      className="flex items-center gap-3 rounded-lg border p-3"
      data-testid={`invite-row-${invitation.invited_email}`}
    >
      <div className="min-w-0 flex-1 space-y-1">
        <div className="text-sm font-medium break-all">{invitation.invited_email}</div>
        <div className="text-sm text-muted-foreground">
          {t("members.sentOn", { date: formatDate(invitation.sent_at) })}
        </div>
        <div className="text-sm text-muted-foreground">
          {t("members.expiresOn", { date: formatDate(invitation.expires_at) })}
        </div>
      </div>
      {menu}
    </div>
  );
}

// The founder's one-time set-password link for a dormant member (ADR-0039). Shown
// once: the plaintext token exists only in this response, so the dialog offers a
// Copy button and never prints the URL (#847).
function ReactivationLinkDialog({
  result,
  onClose,
}: {
  result: ReactivateResp;
  onClose: () => void;
}) {
  const { t } = useTranslation(["settings", "common"]);
  const { copied, copy } = useCopyLink();
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent data-testid="reactivation-dialog">
        <DialogHeader>
          <DialogTitle>{t("members.linkTitle", { email: result.email })}</DialogTitle>
          <DialogDescription>
            {t("members.linkExpires", { when: formatDateTime(result.expires_at) })}{" "}
            {t("members.linkHint")}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type="button"
            data-testid="copy-reactivation-link"
            onClick={() => void copy(result.set_password_url)}
          >
            {copied ? t("members.copied") : t("members.copyLink")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

import { useState } from "react";
import { Archive } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { errorMessage } from "@/lib/errorMessage";
import { useUpdateLifecycle } from "@/hooks/useLifecycle";
import { statusOptions, type LifecycleGroup } from "@/lib/lifecycle";

// todayISO returns YYYY-MM-DD in the local timezone. toISOString() would shift
// users east of UTC into yesterday for the first hours of their day.
function todayISO(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

type Props = {
  group: LifecycleGroup;
  id: string;
  listKey: string;
  currentStatus: string;
  currentTerminatedAt: string | null;
  currentNote: string | null;
};

// Dedicated "change lifecycle status" dialog (ADR-0009): a separate action from
// Edit, operating on the parent table via PATCH /{group}/{id}/lifecycle. The
// backend enforces the biconditional (status=active ⟺ terminated_at IS NULL);
// we mirror it here so the date field appears only when terminating and
// auto-fills today the moment a terminal status is picked.
export function TerminatePositionDialog({
  group,
  id,
  listKey,
  currentStatus,
  currentTerminatedAt,
  currentNote,
}: Props) {
  const { t } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const mutation = useUpdateLifecycle(group, id, listKey);

  const [status, setStatus] = useState(currentStatus);
  const [terminatedAt, setTerminatedAt] = useState(
    currentTerminatedAt ? currentTerminatedAt.slice(0, 10) : "",
  );
  const [note, setNote] = useState(currentNote ?? "");

  const wasActive = currentStatus === "active";
  const isActive = status === "active";

  function reset() {
    setStatus(currentStatus);
    setTerminatedAt(
      currentTerminatedAt ? currentTerminatedAt.slice(0, 10) : "",
    );
    setNote(currentNote ?? "");
    mutation.reset();
  }

  function close() {
    setOpen(false);
    reset();
  }

  function onStatusChange(next: string) {
    setStatus(next);
    // Picking a terminal status auto-fills today (require + default today).
    // Going back to active clears the date so the biconditional holds.
    if (next === "active") {
      setTerminatedAt("");
    } else if (!terminatedAt) {
      setTerminatedAt(todayISO());
    }
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(
      {
        status,
        terminated_at: isActive ? null : terminatedAt,
        termination_note: note.trim() ? note.trim() : null,
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Archive className="mr-1 size-4" />
          {wasActive ? t("terminate.closeTrigger") : t("terminate.editTrigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {wasActive ? t("terminate.closeTitle") : t("terminate.editTitle")}
          </DialogTitle>
          <DialogDescription>{t("terminate.description")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="lifecycle_status">
              {t("terminate.statusLabel")}
            </Label>
            <select
              id="lifecycle_status"
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
              value={status}
              onChange={(e) => onStatusChange(e.target.value)}
            >
              {statusOptions(group).map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </div>

          {!isActive && (
            <div className="grid gap-2">
              <Label htmlFor="lifecycle_terminated_at">
                {t("terminate.terminatedOnLabel")}
              </Label>
              <Input
                id="lifecycle_terminated_at"
                type="date"
                required
                max="9999-12-31"
                value={terminatedAt}
                onChange={(e) => setTerminatedAt(e.target.value)}
              />
            </div>
          )}

          <div className="grid gap-2">
            <Label htmlFor="lifecycle_note">{t("terminate.noteLabel")}</Label>
            <Input
              id="lifecycle_note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder={t("terminate.notePlaceholder")}
            />
          </div>

          {mutation.error && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("actions.saving") : t("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

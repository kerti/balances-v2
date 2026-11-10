import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Callout } from "@/components/common/Callout";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";
import { FormError, ValidatedForm } from "@/components/common/ValidatedForm";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useMoveSnapshot, type MovableSnapshot } from "@/hooks/useMoveSnapshot";
import { yearMonthsUpToNow } from "@/lib/dateLimits";
import { formatCurrency } from "@/lib/format";
import { formatRecordMonth, snapshotForMonth, ymOf } from "@/lib/recordSnapshot";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  snapshot: MovableSnapshot & { as_of_date: string | null };
  /** Called once the move succeeded, so the Edit dialog behind can close too. */
  onMoved: () => void;
};

const MONTH_FIELD = "move_year_month";

// "Move…" from Edit snapshot (ADR-0046 amendment, #791): the month stays
// read-only in the edit form, and moving is its own act. One atomic request
// archives the snapshot, records the same values in the picked month (replacing
// whatever that month held, with the replace notice up front, as in Record
// snapshot), and ends in one toast with Undo. It moves the snapshot as saved:
// edits typed in the Edit form behind this dialog are not part of it.
export function MoveSnapshotDialog({ open, onOpenChange, snapshot, onMoved }: Props) {
  const { t } = useTranslation("common");
  const { snapshots, mutation } = useMoveSnapshot(snapshot);
  const [target, setTarget] = useState("");

  const sameMonth = target !== "" && ymOf(target) === ymOf(snapshot.year_month);
  const taken = target !== "" && !sameMonth ? snapshotForMonth(snapshots, target) : undefined;

  function close() {
    onOpenChange(false);
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(target, {
      onSuccess: () => {
        close();
        onMoved();
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("snapshot.moveTitle")}</DialogTitle>
          <DialogDescription>
            {t("snapshot.moveDescription", { month: formatRecordMonth(snapshot.year_month) })}
          </DialogDescription>
        </DialogHeader>
        <ValidatedForm
          onSubmit={submit}
          error={mutation.error}
          className="space-y-3"
          validate={() => {
            if (target === "") return { [MONTH_FIELD]: t("validation.required") };
            if (sameMonth) return { [MONTH_FIELD]: t("snapshot.moveSameMonth") };
            return {};
          }}
        >
          <div className="grid gap-2">
            <Label htmlFor={MONTH_FIELD}>{t("snapshot.moveTo")}</Label>
            <MonthPickerPopover
              fullWidth
              id={MONTH_FIELD}
              testId={MONTH_FIELD}
              months={yearMonthsUpToNow()}
              selected={target}
              onSelect={setTarget}
              placeholder={t("snapshot.movePlaceholder")}
            />
            {taken && (
              <Callout data-testid="snapshot-move-replace-notice">
                {t("snapshot.moveReplaceNotice", {
                  month: formatRecordMonth(target),
                  value: formatCurrency(taken.amount, taken.currency),
                })}
              </Callout>
            )}
          </div>
          {snapshot.as_of_date && (
            <p className="text-xs text-muted-foreground" data-testid="snapshot-move-date-note">
              {t("snapshot.moveStatementNote", { month: formatRecordMonth(snapshot.year_month) })}
            </p>
          )}

          <FormError />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending} data-testid="snapshot-move-confirm">
              {mutation.isPending ? t("snapshot.moving") : t("snapshot.moveConfirm")}
            </Button>
          </DialogFooter>
        </ValidatedForm>
      </DialogContent>
    </Dialog>
  );
}

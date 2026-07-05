import { useState } from "react";
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
import { errorMessage } from "@/lib/errorMessage";

type Props = {
  title: string;
  description: string;
  /** Footer submit-button label at rest (e.g. Create / Save changes). */
  submitLabel: string;
  /** Footer submit-button label while the mutation is in flight. */
  pendingLabel: string;
  isPending: boolean;
  /**
   * Extra disable condition for the submit button (e.g. an inline validation
   * like time-deposit's term check). ORs with `isPending` for the disabled
   * state only — the button label still tracks `isPending`, so a merely-invalid
   * form keeps its rest label rather than flipping to the pending label.
   */
  submitDisabled?: boolean;
  /** The mutation error, if any; rendered via the shared error envelope. */
  error: unknown;
  /**
   * Called on form submit with a `close` callback. The caller maps its form
   * to a payload and mutates, wiring `onSuccess: close` to dismiss the dialog.
   * The scaffold owns `preventDefault`.
   */
  onSubmit: (close: () => void) => void;
  /** The hand-written form body (fields only — no shell, error, or footer). */
  children: React.ReactNode;
  /** Optional DialogContent class override (e.g. tall investment forms). */
  contentClassName?: string;
  /**
   * Optional `<form>` class override. Defaults to `space-y-3`; the sectioned
   * bond/time-deposit forms pass `space-y-4` to keep their group spacing.
   */
  formClassName?: string;
  // --- Create mode: pass `trigger`; the scaffold owns open state. ---
  /** Trigger element (e.g. the "+ Add" button). Presence selects create mode. */
  trigger?: React.ReactNode;
  /** Called after the dialog closes in create mode — reset form + mutation here. */
  onClosed?: () => void;
  // --- Edit mode: pass controlled `open`/`onOpenChange`, no trigger. ---
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
};

/**
 * Shared scaffold for Position Create/Edit dialogs (ADR-0043 follow-on phase).
 * Owns the dialog shell, header, form wrapper, error envelope, and footer that
 * every per-type dialog duplicated; each type still supplies its own field body
 * as `children`. Two modes: create (self-triggering, owns open state) and edit
 * (controlled via `open`/`onOpenChange`). The dirty-guard is deliberately out of
 * scope — no such guard exists in the codebase yet; it is a separate decision.
 */
export function PositionFormDialog({
  title,
  description,
  submitLabel,
  pendingLabel,
  isPending,
  submitDisabled = false,
  error,
  onSubmit,
  children,
  contentClassName,
  formClassName = "space-y-3",
  trigger,
  onClosed,
  open: controlledOpen,
  onOpenChange,
}: Props) {
  const { t } = useTranslation("common");
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  // Edit dialogs drive open state from the caller; create dialogs own it.
  const isControlled = onOpenChange !== undefined;
  const open = isControlled ? (controlledOpen ?? false) : uncontrolledOpen;

  function close() {
    if (isControlled) {
      onOpenChange?.(false);
    } else {
      setUncontrolledOpen(false);
      onClosed?.();
    }
  }

  function handleOpenChange(next: boolean) {
    if (isControlled) {
      onOpenChange?.(next);
    } else if (next) {
      setUncontrolledOpen(true);
    } else {
      close();
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {trigger && <DialogTrigger asChild>{trigger}</DialogTrigger>}
      <DialogContent className={contentClassName}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            onSubmit(close);
          }}
          className={formClassName}
        >
          {children}

          {error != null && (
            <p className="text-sm text-destructive">{errorMessage(error)}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={isPending || submitDisabled}>
              {isPending ? pendingLabel : submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

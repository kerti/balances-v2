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
import { FormError, ValidatedForm, type FieldMessages } from "@/components/common/ValidatedForm";
import { TagSelect } from "@/components/common/TagSelect";

type Props = {
  title: string;
  description: string;
  /** Footer submit-button label at rest (e.g. Create / Save changes). */
  submitLabel: string;
  /** Footer submit-button label while the mutation is in flight. */
  pendingLabel: string;
  isPending: boolean;
  /**
   * Rules the fields' own `required`/`min`/`max` can't express (e.g. the
   * time-deposit term check), keyed by field id. The submit button is never
   * disabled for an invalid form: clicking it shows these beside their fields
   * (#824, ADR-0058 §5).
   */
  validate?: () => FieldMessages;
  /** The mutation error, if any: beside its field when the code maps to one, else a form-level message. */
  error: unknown;
  /**
   * Called on form submit with a `close` callback. The caller maps its form
   * to a payload and mutates, wiring `onSuccess: close` to dismiss the dialog.
   * The scaffold owns `preventDefault`.
   */
  onSubmit: (close: () => void) => void;
  /**
   * The position's Tag (ADR-0028). Position dialogs pass their tag state and get
   * the one Tag field, in the same slot under the type's own fields in Create and
   * Edit alike (#839); the rate and tag dialogs that share this shell omit it.
   * The caller sends the value as the write's `tag_id`, so the Tag is saved by the
   * one Save, with the one toast.
   */
  tag?: { value: string | null; onChange: (tagId: string | null) => void };
  /** The hand-written form body (fields only — no shell, error, or footer). */
  children: React.ReactNode;
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
  validate,
  error,
  onSubmit,
  tag,
  children,
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
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <ValidatedForm
          onSubmit={() => onSubmit(close)}
          validate={validate}
          error={error}
          className={formClassName}
        >
          {children}

          {tag && <TagSelect value={tag.value} onChange={tag.onChange} />}

          <FormError />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={isPending}>
              {isPending ? pendingLabel : submitLabel}
            </Button>
          </DialogFooter>
        </ValidatedForm>
      </DialogContent>
    </Dialog>
  );
}

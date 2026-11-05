import { useTranslation } from "react-i18next";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  onConfirm: () => void;
  pending?: boolean;
};

// ConfirmDialog wraps shadcn's AlertDialog for the common pattern of
// "are you sure?" prompts before destructive actions. Replaces ad-hoc
// window.confirm calls so the app's design language stays consistent.
// Default Confirm/Cancel/Working labels are resolved via i18n so call sites
// that don't override them still translate (ADR-0026).
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel,
  destructive,
  onConfirm,
  pending,
}: Props) {
  const { t } = useTranslation("common");
  const confirm = confirmLabel ?? t("confirm");
  const cancel = cancelLabel ?? t("cancel");
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          {description && <AlertDialogDescription>{description}</AlertDialogDescription>}
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>{cancel}</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={pending}
            // A variant, not a className: AlertDialogAction renders Button asChild,
            // and Slot joins classes without tailwind-merge, so a bg-* override
            // lost to the default variant's bg-primary (#779).
            variant={destructive ? "destructive" : "default"}
          >
            {pending ? t("working") : confirm}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

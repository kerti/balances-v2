import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  useMenuOpenOnClick,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

type Props = {
  // aria-label on the trigger — descriptor copy, so it names the row's noun.
  label: string;
  onEdit: () => void;
  onDelete: () => void;
  /** An extra first item (e.g. a tag's "Add items…"); both must be given. */
  extraAction?: { label: string; icon: ReactNode; onSelect: () => void };
  /** Trigger button style. The desktop table row uses the bare "ghost" dots;
   *  the mobile card uses "outline" + `size-11` so the action reads as a real
   *  menu button at the tap-target floor — matching `IncomeRowMenu`. */
  variant?: "ghost" | "outline";
  triggerClassName?: string;
};

// The row's ⋮ → edit/delete menu, lifted out of the old per-type `*ListRow`
// files into one component both renderers share (ADR-0043). The edit/delete
// verbs are shared `common` copy; the actual dialogs are owned by the core, so
// this only signals intent upward. `stopPropagation` keeps a menu click from
// selecting the row underneath it.
export function RowActionsMenu({
  label,
  onEdit,
  onDelete,
  extraAction,
  variant = "ghost",
  triggerClassName,
}: Props) {
  const { t } = useTranslation("common");
  // The ⋮ is dead on iOS without this — see `useMenuOpenOnClick` (#572).
  const menu = useMenuOpenOnClick();
  return (
    <DropdownMenu {...menu.rootProps}>
      <DropdownMenuTrigger asChild>
        <Button
          variant={variant}
          size="icon"
          className={cn(triggerClassName)}
          aria-label={label}
          {...menu.triggerProps}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
        {extraAction && (
          <DropdownMenuItem onClick={extraAction.onSelect} data-testid="row-extra-action">
            {extraAction.icon}
            {extraAction.label}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={onEdit}>
          <Pencil />
          {t("actions.edit")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onDelete} variant="destructive">
          <Trash2 />
          {t("delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

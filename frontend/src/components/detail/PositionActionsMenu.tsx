import { Archive, MoreHorizontal, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  useMenuOpenOnClick,
} from "@/components/ui/dropdown-menu";

type Props = {
  // An active position offers "Close…"; a closed one "Change status" (the same
  // dialog, reopened to correct the date or note).
  active: boolean;
  onClose: () => void;
  onDelete: () => void;
};

// The detail header's ⋯ menu (#839): the status change and the delete, behind one
// icon button so the header stays Help · Edit · ⋯ on a single row at every width.
// Delete sits last, below a separator, in destructive red; the confirm + undo
// flow it opens is unchanged (ADR-0058).
export function PositionActionsMenu({ active, onClose, onDelete }: Props) {
  const { t } = useTranslation("common");
  // The trigger is dead on iOS without this — see `useMenuOpenOnClick` (#572).
  const menu = useMenuOpenOnClick();
  return (
    <DropdownMenu {...menu.rootProps}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="icon"
          className="max-md:size-11"
          aria-label={t("positionMenu.label")}
          data-testid="position-menu-trigger"
          {...menu.triggerProps}
        >
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onClose} data-testid="position-menu-close">
          <Archive />
          {active ? t("terminate.closeTrigger") : t("terminate.editTrigger")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          onClick={onDelete}
          data-testid="position-menu-delete"
        >
          <Trash2 />
          {t("delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

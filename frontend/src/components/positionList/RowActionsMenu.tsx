import { useTranslation } from "react-i18next";
import { MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type Props = {
  // aria-label on the trigger — descriptor copy, so it names the row's noun.
  label: string;
  onEdit: () => void;
  onDelete: () => void;
};

// The row's ⋮ → edit/delete menu, lifted out of the old per-type `*ListRow`
// files into one component both renderers share (ADR-0043). The edit/delete
// verbs are shared `common` copy; the actual dialogs are owned by the core, so
// this only signals intent upward. `stopPropagation` keeps a menu click from
// selecting the row underneath it.
export function RowActionsMenu({ label, onEdit, onDelete }: Props) {
  const { t } = useTranslation("common");
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label={label}
          onClick={(e) => e.stopPropagation()}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
        <DropdownMenuItem onClick={onEdit}>
          {t("actions.edit")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onDelete} variant="destructive">
          {t("delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

import { useTranslation } from "react-i18next";
import { MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

type Props = {
  onEdit: () => void;
  onDelete: () => void;
  /** Trigger button style. Desktop table rows use the bare "ghost" dots; mobile
   *  cards use "outline" so the action reads as a real menu button. */
  variant?: "ghost" | "outline";
  triggerClassName?: string;
};

// The snapshot row's ⋮ menu (edit / delete), shared verbatim by the desktop
// table `SnapshotRow` and the mobile `SnapshotCard` so the action surface can't
// drift between renderers (ADR-0051 Phase B). Keeps the `snapshot.rowActions`
// accessible name in both, which the snapshot e2e keys off.
export function SnapshotRowMenu({ onEdit, onDelete, variant = "ghost", triggerClassName }: Props) {
  const { t } = useTranslation("common");
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant={variant}
          size="icon"
          aria-label={t("snapshot.rowActions")}
          className={cn(triggerClassName)}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>{t("actions.edit")}</DropdownMenuItem>
        <DropdownMenuItem onClick={onDelete} variant="destructive">
          {t("delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

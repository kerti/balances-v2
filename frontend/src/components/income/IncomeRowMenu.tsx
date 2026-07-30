import { useTranslation } from "react-i18next";
import { MoreHorizontal } from "lucide-react";
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
  onEdit: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
  /** Trigger button style. Desktop table rows use the bare "ghost" dots;
   *  mobile cards use "outline" so the action reads as a real menu button. */
  variant?: "ghost" | "outline";
  triggerClassName?: string;
};

// The income row's ⋮ menu (edit / duplicate / delete), shared verbatim by the
// desktop table row and the mobile card so the action surface can't drift
// between renderers (ADR-0050). The trigger keeps the `income:rowActions`
// accessible name in both, which the write-flow e2e keys off.
export function IncomeRowMenu({
  onEdit,
  onDuplicate,
  onDelete,
  variant = "ghost",
  triggerClassName,
}: Props) {
  const { t } = useTranslation(["income", "common"]);
  // The ⋮ is dead on iOS without this — see `useMenuOpenOnClick` (#572).
  const menu = useMenuOpenOnClick();
  return (
    <DropdownMenu {...menu.rootProps}>
      <DropdownMenuTrigger asChild>
        <Button
          variant={variant}
          size="icon"
          aria-label={t("income:rowActions")}
          className={cn(triggerClassName)}
          {...menu.triggerProps}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>{t("common:actions.edit")}</DropdownMenuItem>
        <DropdownMenuItem onClick={onDuplicate}>{t("income:actions.duplicate")}</DropdownMenuItem>
        <DropdownMenuItem onClick={onDelete} variant="destructive">
          {t("common:delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

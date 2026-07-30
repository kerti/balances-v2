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
  onDelete: () => void;
  /** Trigger button style. Desktop table rows use the bare "ghost" dots; mobile
   *  cards use "outline" so the action reads as a real menu button. */
  variant?: "ghost" | "outline";
  triggerClassName?: string;
};

// The transaction row's ⋮ menu (edit / delete), shared verbatim by the desktop
// table `TransactionRow` and the mobile `TransactionCard` so the action surface
// can't drift between renderers (ADR-0051 Phase B) — the `SnapshotRowMenu`
// idiom for the ledger shape. Keeps the `transactionRow.actions` accessible name
// in both, which the transaction e2e keys off.
export function TransactionRowMenu({
  onEdit,
  onDelete,
  variant = "ghost",
  triggerClassName,
}: Props) {
  const { t } = useTranslation(["investments", "common"]);
  // The ⋮ is dead on iOS without this — see `useMenuOpenOnClick` (#572).
  const menu = useMenuOpenOnClick();
  return (
    <DropdownMenu {...menu.rootProps}>
      <DropdownMenuTrigger asChild>
        <Button
          variant={variant}
          size="icon"
          aria-label={t("investments:transactionRow.actions")}
          className={cn(triggerClassName)}
          {...menu.triggerProps}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>{t("common:actions.edit")}</DropdownMenuItem>
        <DropdownMenuItem onClick={onDelete} variant="destructive">
          {t("common:delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

import { useState } from "react";
import { useTranslation } from "react-i18next";
import { MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { TableCell, TableRow } from "@/components/ui/table";
import { StatusBadge } from "@/components/StatusBadge";
import { EditPropertyDialog } from "@/components/EditPropertyDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { useDeleteProperty } from "@/hooks/useProperties";
import { formatCurrency, formatYearMonth } from "@/lib/format";
import { isActiveStatus } from "@/lib/lifecycle";
import { cn } from "@/lib/utils";
import type { PropertyListItem } from "@/api/types";

type Props = {
  item: PropertyListItem;
  // Resolved by the screen (nickname ?? display_name, or "Joint").
  ownerLabel: string;
  onSelect: (id: string) => void;
};

export function PropertyListRow({ item, ownerLabel, onSelect }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deleteMutation = useDeleteProperty();

  const terminated = !isActiveStatus(item.asset.status);
  const propertyForEdit = { asset: item.asset, details: item.details };

  function handleConfirmDelete() {
    deleteMutation.mutate(item.asset.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  // property_type is a closed enum (house/apartment/land/commercial) so it
  // translates against the propertyTypes sub-namespace; address is free-form
  // user text and stays as-is.
  const typeLabel = t(
    `assets:property.propertyTypes.${item.details.property_type}`,
  );
  const secondary = [typeLabel, item.details.address]
    .filter(Boolean)
    .join(" · ");

  return (
    <>
      <TableRow
        className={cn("cursor-pointer", terminated && "text-muted-foreground")}
        onClick={() => onSelect(item.asset.id)}
      >
        <TableCell>
          <div className={cn("font-medium", terminated && "font-normal")}>
            {item.asset.display_name}
          </div>
          <div className="text-xs text-muted-foreground">
            {secondary || "—"}
          </div>
        </TableCell>
        <TableCell>{ownerLabel}</TableCell>
        <TableCell>
          <StatusBadge group="assets" status={item.asset.status} />
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {item.latest_snapshot ? (
            <>
              <div>
                {formatCurrency(
                  item.latest_snapshot.amount,
                  item.latest_snapshot.currency,
                )}
              </div>
              <div className="text-xs text-muted-foreground">
                {formatYearMonth(item.latest_snapshot.year_month)}
              </div>
            </>
          ) : (
            <span className="text-muted-foreground">{"—"}</span>
          )}
        </TableCell>
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("assets:property.rowActions")}
                onClick={(e) => e.stopPropagation()}
              >
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              onClick={(e) => e.stopPropagation()}
            >
              <DropdownMenuItem onClick={() => setEditOpen(true)}>
                {t("common:actions.edit")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => setDeleteOpen(true)}
                variant="destructive"
              >
                {t("common:delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

      <EditPropertyDialog
        key={propertyForEdit.asset.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        property={propertyForEdit}
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("assets:property.deleteTitle")}
        description={t("assets:property.deleteRowDescription", {
          name: item.asset.display_name,
        })}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );
}

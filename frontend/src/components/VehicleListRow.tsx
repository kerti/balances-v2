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
import { EditVehicleDialog } from "@/components/EditVehicleDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { useDeleteVehicle } from "@/hooks/useVehicles";
import { formatCurrency, formatYearMonth } from "@/lib/format";
import { isActiveStatus } from "@/lib/lifecycle";
import { cn } from "@/lib/utils";
import type { VehicleListItem } from "@/api/types";

type Props = {
  item: VehicleListItem;
  // Resolved by the screen (nickname ?? display_name, or "Joint").
  ownerLabel: string;
  onSelect: (id: string) => void;
};

export function VehicleListRow({ item, ownerLabel, onSelect }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deleteMutation = useDeleteVehicle();

  const terminated = !isActiveStatus(item.asset.status);
  const vehicleForEdit = { asset: item.asset, details: item.details };

  function handleConfirmDelete() {
    deleteMutation.mutate(item.asset.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  // vehicle_type is a closed enum (car/motorcycle/other) so it translates
  // against the vehicleTypes sub-namespace; make/model/year/plate stay
  // free-form user text.
  const typeLabel = t(
    `assets:vehicle.vehicleTypes.${item.details.vehicle_type}`,
  );
  const makeModel = [item.details.make, item.details.model]
    .filter(Boolean)
    .join(" ");
  const secondary = [
    typeLabel,
    makeModel,
    item.details.year ? String(item.details.year) : null,
    item.details.plate_number,
  ]
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
                aria-label={t("assets:vehicle.rowActions")}
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

      <EditVehicleDialog
        key={vehicleForEdit.asset.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        vehicle={vehicleForEdit}
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("assets:vehicle.deleteTitle")}
        description={t("assets:vehicle.deleteRowDescription", {
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

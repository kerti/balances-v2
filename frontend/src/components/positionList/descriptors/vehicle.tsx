import { CreateVehicleDialog } from "@/components/CreateVehicleDialog";
import { EditVehicleDialog } from "@/components/EditVehicleDialog";
import {
  useVehicles,
  useDeleteVehicle,
  useImportCreateVehicle,
} from "@/hooks/useVehicles";
import { nonInvestmentDescriptor } from "@/components/positionList/presets/nonInvestment";
import type { VehicleListItem } from "@/api/types";

// Vehicle, on the non-investment preset (ADR-0043). vehicle_type is a closed
// enum (car/motorcycle/other) translated against vehicleTypes;
// make/model/year/plate are free-form user text.
export const vehicleDescriptor = nonInvestmentDescriptor<VehicleListItem>({
  entityKey: "vehicle",
  testIdPrefix: "vehicle",
  group: "assets",
  i18nNamespaces: ["assets", "common", "errors"],
  keys: {
    listTitle: "assets:vehicle.listTitle",
    listSubtitle: "assets:vehicle.listSubtitle",
    emptyTitle: "assets:vehicle.emptyTitle",
    emptyBody: "assets:vehicle.emptyBody",
    noun: "assets:vehicle.noun",
    nounPlural: "assets:vehicle.nounPlural",
    valueLabel: "assets:vehicle.sortLatestValuation",
    rowActions: "assets:vehicle.rowActions",
    deleteTitle: "assets:vehicle.deleteTitle",
  },
  useList: useVehicles,
  useDelete: useDeleteVehicle,
  useImport: useImportCreateVehicle,
  entity: (item) => item.asset,
  getSnapshot: (item) => item.latest_snapshot,
  getSecondary: (item, t) => {
    const typeLabel = t(
      `assets:vehicle.vehicleTypes.${item.details.vehicle_type}`,
    );
    const makeModel = [item.details.make, item.details.model]
      .filter(Boolean)
      .join(" ");
    return (
      [
        typeLabel,
        makeModel,
        item.details.year ? String(item.details.year) : null,
        item.details.plate_number,
      ]
        .filter(Boolean)
        .join(" · ") || "—"
    );
  },
  deleteDescription: (item, t) =>
    t("assets:vehicle.deleteRowDescription", {
      name: item.asset.display_name,
    }),
  headlineLabelKey: "assets:vehicle.totalValue",
  headlineTestId: "vehicles-total",
  renderCreateDialog: () => <CreateVehicleDialog />,
  renderEditDialog: (item, props) => (
    <EditVehicleDialog
      key={item.asset.id}
      vehicle={{ asset: item.asset, details: item.details }}
      {...props}
    />
  ),
});

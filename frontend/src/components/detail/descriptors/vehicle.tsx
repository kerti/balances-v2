import { useTranslation } from "react-i18next";
import { TableHead, TableRow } from "@/components/ui/table";
import { SnapshotRow } from "@/components/common/SnapshotRow";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { EditVehicleDialog } from "@/components/dialogs/EditVehicleDialog";
import { useVehicle, useDeleteVehicle } from "@/hooks/useVehicles";
import {
  useSnapshots,
  useCreateSnapshot,
  useUpdateSnapshot,
  useDeleteSnapshot,
  useImportSnapshots,
  importTemplateUrl,
  vehicleExportUrl,
} from "@/hooks/useAssetSnapshots";
import { suggestRevalued } from "@/lib/revaluation";
import type { InfoField } from "@/components/detail/types";
import type { Vehicle } from "@/api/types";
import type { DetailDescriptor } from "@/components/detail/types";

// Vehicle detail, on the generic `PositionDetailScreen` (ADR-0051, A2). Same
// amount-only shape as Property; the depreciation line folds into `InfoGrid`, and
// the revaluation `suggest` negates the stored (positive) depreciation % so the
// helper reads it as a decline.
export const vehicleDescriptor: DetailDescriptor<Vehicle> = {
  entityKey: "vehicle",
  testIdPrefix: "vehicle",
  group: "assets",
  tagGroup: "asset",
  listKey: "vehicles",
  i18nNamespaces: ["assets", "common", "errors"],
  keys: {
    detailsCardTitle: "assets:vehicle.detailsCardTitle",
    detailsCardLine: "assets:vehicle.detailsCardLine",
    chartTitle: "assets:vehicle.chartTitle",
    chartDescription: "assets:vehicle.chartDescription",
    snapshotsTitle: "assets:vehicle.snapshotsTitle",
    snapshotsDescription: "assets:vehicle.snapshotsDescription",
    snapshotsEmpty: "assets:vehicle.snapshotsEmpty",
    deleteTitle: "assets:vehicle.deleteTitle",
    deleteDescription: "assets:vehicle.deleteDetailDescription",
  },
  tourKeyPrefix: "assets:vehicle",

  useEntity: useVehicle,
  useDelete: useDeleteVehicle,
  exportUrl: vehicleExportUrl,
  getAsset: (entity) => entity.asset,

  headerSecondary: (entity, _ctx, t) => {
    const { details } = entity;
    const makeModel = [details.make, details.model].filter(Boolean).join(" ");
    return [
      t(`assets:vehicle.vehicleTypes.${details.vehicle_type}`),
      makeModel,
      details.year ? String(details.year) : null,
      details.plate_number,
    ]
      .filter(Boolean)
      .join(" · ");
  },
  infoFields: (entity, _ctx, t) => {
    const fields: InfoField[] = [];
    if (entity.details.annual_depreciation_rate) {
      fields.push({
        label: t("assets:vehicle.depreciationRateLabel"),
        value: t("assets:vehicle.depreciationRateValue", {
          rate: Number(entity.details.annual_depreciation_rate).toFixed(2),
        }),
      });
    }
    return fields;
  },

  snapshot: {
    useSectionRender: (assetId) => {
      const { t } = useTranslation(["common"]);
      const { data: vehicle } = useVehicle(assetId);
      const { data: snapshots } = useSnapshots(assetId);
      const createMutation = useCreateSnapshot(assetId);
      const updateMutation = useUpdateSnapshot(assetId);
      const deleteMutation = useDeleteSnapshot(assetId);
      const importMutation = useImportSnapshots(assetId);
      return {
        snapshots,
        header: (
          <TableRow>
            <TableHead>{t("common:tableHeaders.month")}</TableHead>
            <TableHead>{t("common:tableHeaders.amount")}</TableHead>
            <TableHead>{t("common:tableHeaders.notes")}</TableHead>
            <TableHead className="w-12"></TableHead>
          </TableRow>
        ),
        renderRow: (snapshot) => (
          <SnapshotRow
            key={snapshot.id}
            snapshot={snapshot}
            updateMutation={updateMutation}
            deleteMutation={deleteMutation}
          />
        ),
        renderCreateControls: (currency) => (
          <>
            <CreateSnapshotDialog
              currency={currency}
              mutation={createMutation}
              suggest={(yearMonth) =>
                suggestRevalued({
                  newYearMonth: yearMonth,
                  // Vehicle stores positive depreciation %; the helper wants
                  // signed (negative = decline), so negate at the callsite.
                  annualRatePct: vehicle?.details.annual_depreciation_rate
                    ? `-${vehicle.details.annual_depreciation_rate}`
                    : null,
                  snapshots,
                })
              }
              carryover={
                snapshots?.[0]
                  ? { amount: snapshots[0].amount, lastSnapshotMonth: snapshots[0].year_month }
                  : null
              }
            />
            <ImportSnapshotsDialog
              templateUrl={importTemplateUrl(assetId)}
              mutation={importMutation}
              currency={currency}
            />
          </>
        ),
      };
    },
  },

  renderEditDialog: (entity, props) => (
    <EditVehicleDialog key={entity.asset.id} vehicle={entity} {...props} />
  ),
};

import { useTranslation } from "react-i18next";
import { TableHead, TableRow } from "@/components/ui/table";
import { SnapshotRow } from "@/components/common/SnapshotRow";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { EditPropertyDialog } from "@/components/dialogs/EditPropertyDialog";
import { useProperty, useDeleteProperty } from "@/hooks/useProperties";
import {
  useSnapshots,
  useCreateSnapshot,
  useUpdateSnapshot,
  useDeleteSnapshot,
  useImportSnapshots,
  importTemplateUrl,
  propertyExportUrl,
} from "@/hooks/useAssetSnapshots";
import { formatCurrency, formatDate, formatSignedPercent } from "@/lib/format";
import { suggestRevalued } from "@/lib/revaluation";
import type { InfoField } from "@/components/detail/types";
import type { Property } from "@/api/types";
import type { DetailDescriptor } from "@/components/detail/types";

// Property detail, expressed on the generic `PositionDetailScreen` (ADR-0051,
// A2). An amount-only asset like the BankAccount linchpin — S1 snapshot rows, no
// transaction sections — but with real info-card content: the acquisition and
// appreciation lines fold into the shared `InfoGrid` as label/value pairs, and
// the snapshot create dialog carries a revaluation `suggest` (the annual
// appreciation rate lives on `details`, so the snapshot hook reads it back).
export const propertyDescriptor: DetailDescriptor<Property> = {
  entityKey: "property",
  testIdPrefix: "property",
  group: "assets",
  tagGroup: "asset",
  listKey: "properties",
  i18nNamespaces: ["assets", "common", "errors"],
  keys: {
    detailsCardTitle: "assets:property.detailsCardTitle",
    detailsCardLine: "assets:property.detailsCardLine",
    chartTitle: "assets:property.chartTitle",
    chartDescription: "assets:property.chartDescription",
    snapshotsTitle: "assets:property.snapshotsTitle",
    snapshotsDescription: "assets:property.snapshotsDescription",
    snapshotsEmpty: "assets:property.snapshotsEmpty",
    deleteTitle: "assets:property.deleteTitle",
    deleteDescription: "assets:property.deleteDetailDescription",
  },
  tourKeyPrefix: "assets:property",

  useEntity: useProperty,
  useDelete: useDeleteProperty,
  exportUrl: propertyExportUrl,
  getAsset: (entity) => entity.asset,

  headerSecondary: (entity, _ctx, t) =>
    [t(`assets:property.propertyTypes.${entity.details.property_type}`), entity.details.address]
      .filter(Boolean)
      .join(" · "),
  infoFields: (entity, _ctx, t) => {
    const { asset, details } = entity;
    const fields: InfoField[] = [];
    if (details.acquisition_date) {
      fields.push({
        label: t("assets:property.acquiredLine"),
        value: details.acquisition_cost
          ? t("assets:property.acquiredForValue", {
              date: formatDate(details.acquisition_date),
              cost: formatCurrency(details.acquisition_cost, asset.native_currency),
            })
          : formatDate(details.acquisition_date),
      });
    }
    if (details.annual_appreciation_rate) {
      fields.push({
        label: t("assets:property.appreciationRateLabel"),
        value: t("assets:property.appreciationRateValue", {
          value: formatSignedPercent(details.annual_appreciation_rate),
        }),
      });
    }
    return fields;
  },

  snapshot: {
    useSectionRender: (assetId) => {
      const { t } = useTranslation(["common"]);
      const { data: property } = useProperty(assetId);
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
                  annualRatePct: property?.details.annual_appreciation_rate ?? null,
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
    <EditPropertyDialog key={entity.asset.id} property={entity} {...props} />
  ),
};

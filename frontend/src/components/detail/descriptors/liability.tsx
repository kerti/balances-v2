import { useTranslation } from "react-i18next";
import { TableHead, TableRow } from "@/components/ui/table";
import { SnapshotRow } from "@/components/common/SnapshotRow";
import { SnapshotCard } from "@/components/common/SnapshotCard";
import { IdentityCluster } from "@/components/detail/IdentityCluster";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { EditLiabilityDialog } from "@/components/dialogs/EditLiabilityDialog";
import { useLiability, useDeleteLiability } from "@/hooks/useLiabilities";
import {
  useLiabilitySnapshots,
  useCreateLiabilitySnapshot,
  useUpdateLiabilitySnapshot,
  useDeleteLiabilitySnapshot,
  useImportLiabilitySnapshots,
  liabilityImportTemplateUrl,
  liabilityExportUrl,
} from "@/hooks/useLiabilitySnapshots";
import type { TourStep } from "@/components/shell/HelpTourButton";
import { formatCurrency, formatDate } from "@/lib/format";
import type { InfoField } from "@/components/detail/types";
import type { Liability, LiabilitySnapshot } from "@/api/types";
import type { DetailDescriptor } from "@/components/detail/types";

// The five standard tour anchors, keyed off the `liabilities` namespace root
// (`tour.*`) rather than a subtype sub-key, so they don't fit the core's
// `${tourKeyPrefix}.tour.${step}` template and are supplied explicitly.
const TOUR_STEPS = ["overview", "actions", "details", "chart", "snapshots"] as const;

// Liability detail, on the generic `PositionDetailScreen` (ADR-0051, A2 —
// cross-group). Unlike the asset types the entity is **flat** (its Position
// fields sit directly on the row, not under an `asset` key), so `getAsset` is
// identity; the principal/interest/term/period lines fold into the shared
// `InfoGrid`. Amount-only snapshots (S1 rows), no transaction sections.
export const liabilityDescriptor: DetailDescriptor<Liability, void, LiabilitySnapshot> = {
  entityKey: "liability",
  testIdPrefix: "liability",
  group: "liabilities",
  tagGroup: "liability",
  listKey: "liabilities",
  i18nNamespaces: ["liabilities", "common", "errors"],
  keys: {
    detailsCardTitle: "liabilities:detailsCardTitle",
    detailsCardLine: "liabilities:detailsCardLine",
    chartTitle: "liabilities:chartTitle",
    chartDescription: "liabilities:chartDescription",
    snapshotsTitle: "liabilities:snapshotsTitle",
    snapshotsDescription: "liabilities:snapshotsDescription",
    snapshotsEmpty: "liabilities:snapshotsEmpty",
    deleteTitle: "liabilities:deleteTitle",
    deleteDescription: "liabilities:deleteDetailDescription",
  },
  tourKeyPrefix: "liabilities",
  tourSteps: (t) =>
    TOUR_STEPS.map((step): TourStep => ({
      element: `[data-testid="tour-${step}"]`,
      title: t(`liabilities:tour.${step}Title`),
      description: t(`liabilities:tour.${step}Body`),
    })),

  useEntity: useLiability,
  useDelete: useDeleteLiability,
  exportUrl: liabilityExportUrl,
  getAsset: (entity) => entity,

  identityCluster: (entity, _ctx, t) => (
    <IdentityCluster
      lines={[entity.counterparty_name, t(`liabilities:subtypes.${entity.subtype}`)]}
    />
  ),
  infoFields: (entity, _ctx, t) => {
    const fields: InfoField[] = [];
    if (entity.principal) {
      fields.push({
        label: t("liabilities:principalLabel"),
        value: formatCurrency(entity.principal, entity.native_currency),
      });
    }
    if (entity.interest_rate) {
      fields.push({
        label: t("liabilities:interestRateLabel"),
        value: t("liabilities:interestRateValue", {
          rate: Number(entity.interest_rate).toFixed(2),
        }),
      });
    }
    if (entity.term_months !== null) {
      fields.push({
        label: t("liabilities:termLabel"),
        value: t("liabilities:termValue", { count: entity.term_months }),
      });
    }
    if (entity.start_date || entity.maturity_date) {
      const periodMissing = t("liabilities:periodMissing");
      fields.push({
        label: t("liabilities:periodLabel"),
        value: t("liabilities:periodValue", {
          start: entity.start_date ? formatDate(entity.start_date) : periodMissing,
          end: entity.maturity_date ? formatDate(entity.maturity_date) : periodMissing,
        }),
      });
    }
    return fields;
  },

  snapshot: {
    useSectionRender: (assetId) => {
      const { t } = useTranslation(["common"]);
      const { data: snapshots } = useLiabilitySnapshots(assetId);
      const createMutation = useCreateLiabilitySnapshot(assetId);
      const updateMutation = useUpdateLiabilitySnapshot(assetId);
      const deleteMutation = useDeleteLiabilitySnapshot(assetId);
      const importMutation = useImportLiabilitySnapshots(assetId);
      return {
        snapshots,
        header: (
          <TableRow>
            <TableHead>{t("common:tableHeaders.month")}</TableHead>
            <TableHead className="text-right">{t("common:tableHeaders.amount")}</TableHead>
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
        renderCard: (snapshot) => (
          <SnapshotCard
            key={snapshot.id}
            snapshot={snapshot}
            updateMutation={updateMutation}
            deleteMutation={deleteMutation}
          />
        ),
        renderCreateControls: (currency) => (
          <CreateSnapshotDialog
            currency={currency}
            mutation={createMutation}
            carryover={
              snapshots?.[0]
                ? { amount: snapshots[0].amount, lastSnapshotMonth: snapshots[0].year_month }
                : null
            }
          />
        ),
        renderImportControl: (currency) => (
          <ImportSnapshotsDialog
            templateUrl={liabilityImportTemplateUrl(assetId)}
            mutation={importMutation}
            currency={currency}
          />
        ),
      };
    },
  },

  renderEditDialog: (entity, props) => (
    <EditLiabilityDialog key={entity.id} liability={entity} {...props} />
  ),
};

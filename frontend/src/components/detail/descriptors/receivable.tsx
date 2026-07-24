import { useTranslation } from "react-i18next";
import { TableHead, TableRow } from "@/components/ui/table";
import { SnapshotRow } from "@/components/common/SnapshotRow";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { EditReceivableDialog } from "@/components/dialogs/EditReceivableDialog";
import { useReceivable, useDeleteReceivable } from "@/hooks/useReceivables";
import {
  useReceivableSnapshots,
  useCreateReceivableSnapshot,
  useUpdateReceivableSnapshot,
  useDeleteReceivableSnapshot,
  useImportReceivableSnapshots,
  receivableImportTemplateUrl,
  receivableExportUrl,
} from "@/hooks/useReceivableSnapshots";
import type { TourStep } from "@/components/shell/HelpTourButton";
import { formatDate } from "@/lib/format";
import type { Receivable, ReceivableSnapshot } from "@/api/types";
import type { DetailDescriptor } from "@/components/detail/types";

// The five standard tour anchors, keyed off the `receivables` namespace root
// (`tour.*`), supplied explicitly like the liability descriptor.
const TOUR_STEPS = ["overview", "actions", "details", "chart", "snapshots"] as const;

// Receivable detail, on the generic `PositionDetailScreen` (ADR-0051, A2 —
// cross-group). Flat entity like Liability, so `getAsset` is identity. No
// info-card fields — the counterparty/due-date ride the header line and the only
// body content is the shared-surface description (rendered by the core). S1
// snapshots, no transaction sections.
export const receivableDescriptor: DetailDescriptor<Receivable, void, ReceivableSnapshot> = {
  entityKey: "receivable",
  testIdPrefix: "receivable",
  group: "receivables",
  tagGroup: "receivable",
  listKey: "receivables",
  i18nNamespaces: ["receivables", "common", "errors"],
  keys: {
    detailsCardTitle: "receivables:detailsCardTitle",
    detailsCardLine: "receivables:detailsCardLine",
    chartTitle: "receivables:chartTitle",
    chartDescription: "receivables:chartDescription",
    snapshotsTitle: "receivables:snapshotsTitle",
    snapshotsDescription: "receivables:snapshotsDescription",
    snapshotsEmpty: "receivables:snapshotsEmpty",
    deleteTitle: "receivables:deleteTitle",
    deleteDescription: "receivables:deleteDetailDescription",
  },
  tourKeyPrefix: "receivables",
  tourSteps: (t) =>
    TOUR_STEPS.map((step): TourStep => ({
      element: `[data-testid="tour-${step}"]`,
      title: t(`receivables:tour.${step}Title`),
      description: t(`receivables:tour.${step}Body`),
    })),

  useEntity: useReceivable,
  useDelete: useDeleteReceivable,
  exportUrl: receivableExportUrl,
  getAsset: (entity) => entity,

  headerSecondary: (entity, _ctx, t) =>
    entity.due_date
      ? t("receivables:detailSubtitleWithDue", {
          counterparty: entity.counterparty_name,
          date: formatDate(entity.due_date),
        })
      : entity.counterparty_name,
  infoFields: () => [],

  snapshot: {
    useSectionRender: (assetId) => {
      const { t } = useTranslation(["common"]);
      const { data: snapshots } = useReceivableSnapshots(assetId);
      const createMutation = useCreateReceivableSnapshot(assetId);
      const updateMutation = useUpdateReceivableSnapshot(assetId);
      const deleteMutation = useDeleteReceivableSnapshot(assetId);
      const importMutation = useImportReceivableSnapshots(assetId);
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
              carryover={
                snapshots?.[0]
                  ? { amount: snapshots[0].amount, lastSnapshotMonth: snapshots[0].year_month }
                  : null
              }
            />
            <ImportSnapshotsDialog
              templateUrl={receivableImportTemplateUrl(assetId)}
              mutation={importMutation}
              currency={currency}
            />
          </>
        ),
      };
    },
  },

  renderEditDialog: (entity, props) => (
    <EditReceivableDialog key={entity.id} receivable={entity} {...props} />
  ),
};
